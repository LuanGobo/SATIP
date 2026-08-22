package app

import (
	"fmt"
	"math/rand"

	"device_app/internal/domain"
)

type Engine struct {
	model *Model

	tSeconds float64
	step     int

	rng   *rand.Rand
	noise map[string]float64
}

type StepResult struct {
	TimeSeconds float64
	Step        int

	Tanks   map[string]TankSnapshot
	Valves  map[string]ValveSnapshot
	Motors  map[string]MotorSnapshot
	Sensors map[string]SensorSnapshot

	Events []domain.Event
}

type SensorSnapshot struct {
	Value    float64
	Deadband float64
	AlarmLL  bool
	AlarmL   bool
	AlarmH   bool
	AlarmHH  bool
	EnabledLL bool
	EnabledL  bool
	EnabledH  bool
	EnabledHH bool
}

type ValveSnapshot struct {
	Open     bool
	Flow     bool
	Position float64
}

type MotorSnapshot struct {
	State  string
	Active bool
	RPM    float64
}

type TankSnapshot struct {
	VolumeL      float64
	LevelPct     float64
	TempC        float64
	PressureBar  float64
	CleanAirNlpm float64
	InflowLPM    float64
}

func NewEngine(m *Model) (*Engine, error) {
	if m == nil {
		return nil, fmt.Errorf("engine: model is nil")
	}
	if m.DTSeconds <= 0 {
		return nil, fmt.Errorf("engine: DTSeconds must be > 0")
	}
	return &Engine{
		model: m,
		noise: map[string]float64{},
	}, nil
}

// SetRNG habilita a variação natural de vazão nas linhas da planta.
func (e *Engine) SetRNG(r *rand.Rand) {
	e.rng = r
}

// Step executa 1 tick da simulação.
func (e *Engine) Step() (StepResult, error) {
	dt := e.model.DTSeconds

	// 1) Resetar inputs "instantâneos" por tick
	for _, tk := range e.model.Tanks {
		tk.SetInFlow(0, tk.TempC())  // sem entrada por padrão; temp default = temp atual
		tk.SetSteamJacket(0, 0)      // steam off
		tk.SetCleanAirInflow(0)      // sem ar limpo
		tk.SetVentOpen(0)            // vent fechado por padrão (vai abrir se conexão ativa)
		tk.SetAgitation(false)       // agitação off por padrão (vai ligar se motor ativo)
	}

	// 1b) Avançar o curso dos atuadores das válvulas em direção ao estado
	// comandado pelo controlador neste tick.
	for _, v := range e.model.Valves {
		v.Tick(dt)
	}

	// 2) Interpretar conexões e aplicar comandos aos equipamentos
	// Para mistura de entradas no mesmo tanque, acumulamos por tanque:
	type inflowAcc struct {
		totalFlowLpm float64
		tempWeighted float64 // soma de (Q*T)
	}
	inflows := map[string]*inflowAcc{} // tankID -> acc

	for _, c := range e.model.Connections {
		switch c.Type {

		case "product_inlet", "cip_inlet":
			factor, err := e.pathFactor(c.Path)
			if err != nil {
				return StepResult{}, err
			}
			if factor <= 0 {
				continue
			}

			tankID := c.Target.EquipmentID
			acc := inflows[tankID]
			if acc == nil {
				acc = &inflowAcc{}
				inflows[tankID] = acc
			}

			q := c.Source.FlowLpm * factor * e.flowNoise(c.ID, dt)
			if q < 0 {
				q = 0
			}
			acc.totalFlowLpm += q
			acc.tempWeighted += q * c.Source.TempC

		case "steam_jacket":
			factor, err := e.pathFactor(c.Path)
			if err != nil {
				return StepResult{}, err
			}

			tk := e.model.Tanks[c.Target.EquipmentID]
			if tk == nil {
				return StepResult{}, fmt.Errorf("engine: steam_jacket target tank '%s' not found", c.Target.EquipmentID)
			}
			tk.SetSteamJacket(factor, c.Source.TempC)

		case "clean_air_inlet":
			factor, err := e.pathFactor(c.Path)
			if err != nil {
				return StepResult{}, err
			}
			if factor <= 0 {
				continue
			}

			tk := e.model.Tanks[c.Target.EquipmentID]
			if tk == nil {
				return StepResult{}, fmt.Errorf("engine: clean_air_inlet target tank '%s' not found", c.Target.EquipmentID)
			}
			tk.SetCleanAirInflow(c.Source.FlowNlpm * factor * e.flowNoise(c.ID, dt))

		case "vent_outlet":
			factor, err := e.pathFactor(c.Path)
			if err != nil {
				return StepResult{}, err
			}

			tk := e.model.Tanks[c.Target.EquipmentID]
			if tk == nil {
				return StepResult{}, fmt.Errorf("engine: vent_outlet target tank '%s' not found", c.Target.EquipmentID)
			}
			tk.SetVentOpen(factor)

		case "agitation_link":
			// path é opcional aqui (ligação mecânica)
			motor := e.model.Motors[c.Source.MotorID]
			if motor == nil {
				return StepResult{}, fmt.Errorf("engine: agitation_link motor '%s' not found", c.Source.MotorID)
			}
			tk := e.model.Tanks[c.Target.EquipmentID]
			if tk == nil {
				return StepResult{}, fmt.Errorf("engine: agitation_link target tank '%s' not found", c.Target.EquipmentID)
			}

			// aplica carga ao motor baseado no volume
			hasLoad := tk.VolumeL() > c.Source.RequiresVolumeGtL
			motor.SetLoad(hasLoad)

			// tanque agita se motor está ativo
			tk.SetAgitation(motor.IsActive())

		case "transfer_outlet":
			tk := e.model.Tanks[c.Source.FromTankID]
			if tk == nil {
				return StepResult{}, fmt.Errorf("engine: transfer_outlet from_tank '%s' not found", c.Source.FromTankID)
			}
			// Set load on motors in the path based on tank volume
			for _, pid := range c.Path {
				if m, ok := e.model.Motors[pid]; ok {
					m.SetLoad(tk.VolumeL() > 1.0)
				}
			}
			factor, err := e.pathFactor(c.Path)
			if err != nil {
				return StepResult{}, err
			}
			if factor <= 0 {
				continue
			}
			outflowL := (c.Source.NominalFlowLpm * factor * e.flowNoise(c.ID, dt) / 60.0) * dt
			tk.RemoveVolume(outflowL)

		default:
			return StepResult{}, fmt.Errorf("engine: unsupported connection type '%s' (%s)", c.Type, c.ID)
		}
	}

	// 3) Aplicar inflows acumulados em cada tanque (com mistura de temperatura)
	for tankID, acc := range inflows {
		tk := e.model.Tanks[tankID]
		if tk == nil {
			return StepResult{}, fmt.Errorf("engine: inflow target tank '%s' not found", tankID)
		}
		if acc.totalFlowLpm <= 0 {
			continue
		}

		mixedTemp := tk.TempC()
		mixedTemp = acc.tempWeighted / acc.totalFlowLpm

		tk.SetInFlow(acc.totalFlowLpm, mixedTemp)
	}

	// 4) Avançar física dos tanques
	for _, tk := range e.model.Tanks {
		tk.Tick(dt)
	}

	// 5) Rodar sensores
	var events []domain.Event
	for _, s := range e.model.Sensors {
		ev := s.Tick(dt)
		if len(ev) > 0 {
			events = append(events, ev...)
		}
	}

	// 5b) Snapshot sensores
	sensorsSnap := make(map[string]SensorSnapshot, len(e.model.Sensors))
	for id, s := range e.model.Sensors {
		sensorsSnap[id] = SensorSnapshot{
			Value:     s.Value(),
			Deadband:  s.Deadband(),
			AlarmLL:   s.AlarmActiveLL(),
			AlarmL:    s.AlarmActiveL(),
			AlarmH:    s.AlarmActiveH(),
			AlarmHH:   s.AlarmActiveHH(),
			EnabledLL: s.AlarmEnabledLL(),
			EnabledL:  s.AlarmEnabledL(),
			EnabledH:  s.AlarmEnabledH(),
			EnabledHH: s.AlarmEnabledHH(),
		}
	}

	// 6) Snapshot (Tanks + Valves + Motors)
	tanksSnap := make(map[string]TankSnapshot, len(e.model.Tanks))
	for id, tk := range e.model.Tanks {
		tanksSnap[id] = TankSnapshot{
			VolumeL:     tk.VolumeL(),
			LevelPct:    tk.LevelPct(),
			TempC:       tk.TempC(),
			PressureBar: tk.PressureBar(),
			// Se você quiser salvar também CleanAir/Inflow aqui,
			// crie getters no Tank e preencha.
		}
	}

	valvesSnap := make(map[string]ValveSnapshot, len(e.model.Valves))
	for id, v := range e.model.Valves {
		valvesSnap[id] = ValveSnapshot{
			Open:     v.IsOpen(),
			Flow:     v.HasFlow(),
			Position: v.Position(),
		}
	}

	motorsSnap := make(map[string]MotorSnapshot, len(e.model.Motors))
	for id, m := range e.model.Motors {
		motorsSnap[id] = MotorSnapshot{
			State:  m.State().String(),
			Active: m.IsActive(),
			RPM:    m.RPM(),
		}
	}

	res := StepResult{
		TimeSeconds: e.tSeconds,
		Step:        e.step,
		Tanks:       tanksSnap,
		Valves:      valvesSnap,
		Motors:      motorsSnap,
		Sensors:     sensorsSnap,
		Events:      events,
	}

	e.step++
	e.tSeconds += dt

	return res, nil
}

// pathFactor avalia o quanto o caminho permite de fluxo, entre 0 e 1.
// Regra atual:
// - Valve: contribui com sua abertura efetiva (curso do atuador)
// - Motor: contribui com 1 se ativo, 0 caso contrário
// O resultado é o produto das contribuições, de modo que qualquer elemento
// fechado ou parado zera o caminho, e uma válvula em curso produz vazão
// parcial.
func (e *Engine) pathFactor(path []string) (float64, error) {
	factor := 1.0
	for _, id := range path {
		if v, ok := e.model.Valves[id]; ok {
			factor *= v.Position()
			continue
		}
		if m, ok := e.model.Motors[id]; ok {
			if !m.IsActive() {
				return 0, nil
			}
			continue
		}
		return 0, fmt.Errorf("engine: path element '%s' not found (valve/motor)", id)
	}
	if factor < 0 {
		factor = 0
	}
	return factor, nil
}

// flowNoise devolve um multiplicador de vazão que varia lentamente ao redor
// de 1, representando as pequenas oscilações de pressão a montante presentes
// em qualquer linha real. É um passeio aleatório amortecido, o que produz
// variação suave em vez de ruído branco ponto a ponto.
func (e *Engine) flowNoise(connID string, dt float64) float64 {
	if e.rng == nil {
		return 1.0
	}
	n := e.noise[connID]
	n += (-n*0.05 + (e.rng.Float64()*2-1)*0.05) * dt
	if n > 0.07 {
		n = 0.07
	}
	if n < -0.07 {
		n = -0.07
	}
	e.noise[connID] = n
	return 1.0 + n
}