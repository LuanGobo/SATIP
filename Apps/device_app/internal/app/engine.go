package app

import (
	"fmt"

	"device_app/internal/domain"
)

type Engine struct {
	model *Model

	tSeconds float64
	step     int
}

type StepResult struct {
	TimeSeconds float64
	Step        int

	Tanks  map[string]TankSnapshot
	Valves map[string]ValveSnapshot
	Motors map[string]MotorSnapshot

	Events []domain.Event
}

type ValveSnapshot struct {
	Open bool
	Flow bool
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
	return &Engine{model: m}, nil
}

// Step executa 1 tick da simulação.
func (e *Engine) Step() (StepResult, error) {
	dt := e.model.DTSeconds

	// 1) Resetar inputs "instantâneos" por tick
	for _, tk := range e.model.Tanks {
		tk.SetInFlow(0, tk.TempC())  // sem entrada por padrão; temp default = temp atual
		tk.SetSteamJacket(false, 0)  // steam off
		tk.SetCleanAirInflow(0)      // sem ar limpo
		tk.SetVentOpen(false)        // vent fechado por padrão (vai abrir se conexão ativa)
		tk.SetAgitation(false)       // agitação off por padrão (vai ligar se motor ativo)
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
			active, err := e.isPathActive(c.Path)
			if err != nil {
				return StepResult{}, err
			}
			if !active {
				continue
			}

			tankID := c.Target.EquipmentID
			acc := inflows[tankID]
			if acc == nil {
				acc = &inflowAcc{}
				inflows[tankID] = acc
			}

			q := c.Source.FlowLpm
			if q < 0 {
				q = 0
			}
			acc.totalFlowLpm += q
			acc.tempWeighted += q * c.Source.TempC

		case "steam_jacket":
			active, err := e.isPathActive(c.Path)
			if err != nil {
				return StepResult{}, err
			}
			if !active {
				continue
			}

			tk := e.model.Tanks[c.Target.EquipmentID]
			if tk == nil {
				return StepResult{}, fmt.Errorf("engine: steam_jacket target tank '%s' not found", c.Target.EquipmentID)
			}
			tk.SetSteamJacket(true, c.Source.TempC)

		case "clean_air_inlet":
			active, err := e.isPathActive(c.Path)
			if err != nil {
				return StepResult{}, err
			}
			if !active {
				continue
			}

			tk := e.model.Tanks[c.Target.EquipmentID]
			if tk == nil {
				return StepResult{}, fmt.Errorf("engine: clean_air_inlet target tank '%s' not found", c.Target.EquipmentID)
			}
			tk.SetCleanAirInflow(c.Source.FlowNlpm)

		case "vent_outlet":
			active, err := e.isPathActive(c.Path)
			if err != nil {
				return StepResult{}, err
			}
			if !active {
				continue
			}

			tk := e.model.Tanks[c.Target.EquipmentID]
			if tk == nil {
				return StepResult{}, fmt.Errorf("engine: vent_outlet target tank '%s' not found", c.Target.EquipmentID)
			}
			tk.SetVentOpen(true)

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
			// Stub: calcula condição de transferência, mas não altera volume ainda.
			_, err := e.isPathActive(c.Path)
			if err != nil {
				return StepResult{}, err
			}
			// Próxima etapa: se ativo => tk.RemoveOutflow(...) / deslocar massa etc.

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
			Open: v.IsOpen(),
			Flow: v.HasFlow(),
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
		Events:      events,
	}

	e.step++
	e.tSeconds += dt

	return res, nil
}

// isPathActive avalia se todos os equipamentos no path permitem fluxo/efeito.
// Regra atual:
// - Valve: precisa estar aberta
// - Motor: precisa estar ativo (IsActive())
func (e *Engine) isPathActive(path []string) (bool, error) {
	for _, id := range path {
		if v, ok := e.model.Valves[id]; ok {
			if !v.IsOpen() {
				return false, nil
			}
			continue
		}
		if m, ok := e.model.Motors[id]; ok {
			if !m.IsActive() {
				return false, nil
			}
			continue
		}
		return false, fmt.Errorf("engine: path element '%s' not found (valve/motor)", id)
	}
	return true, nil
}