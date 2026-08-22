package domain

import (
	"fmt"
)

type Tank struct {
	id  string
	tag string

	tempC        float64
	pressureBar  float64

	// capacidade do tanque
	capacityL float64
	volumeL   float64

	// Entradas no tanque
	inflowLpm    float64 // L/min
	inFlowTempC float64 // °C

	// Headspace / pressão
	cleanAirFlowNlpm float64
	ventOpening      float64 // abertura efetiva do respiro (0 a 1)

	// Agitação
	agitationON bool

	// Vapor
	steamOpening     float64 // abertura efetiva da válvula de vapor (0 a 1)
	steamSupplyTempC float64 // temperatura da fonte de vapor (ex: 140°C)
	steamTempC       float64 // temperatura real da camisa (dinâmica)

	// Ambiente
	ambientTempC float64

	// Parâmetros térmicos
	uaWPerk        float64
	cpJPerKgK      float64
	densityKgPerL  float64
	jacketCoolTau  float64

	// Parâmetros de pressão
	gasGainCoeff    float64
	ventReliefCoeff float64
}

func NewTank(
	id, tag string,
	capacityL, initialVolumeL, initialTempC float64,
) (*Tank, error) {

	if id == "" {
		return nil, fmt.Errorf("tank: id is required")
	}
	if capacityL <= 0 {
		return nil, fmt.Errorf("tank: capacityL must be > 0")
	}
	if initialVolumeL < 0 || initialVolumeL > capacityL {
		return nil, fmt.Errorf("tank: initialVolumeL must be between 0 and capacityL")
	}

	t := &Tank{
		id:            id,
		tag:           tag,
		capacityL:     capacityL,
		volumeL:       initialVolumeL,
		tempC:         initialTempC,
		pressureBar:   1.0,

		inFlowTempC:   initialTempC,
		steamTempC:    initialTempC, // camisa em equilíbrio com o ambiente
		ambientTempC:  initialTempC,

		uaWPerk:        400.0,
		cpJPerKgK:      4180.0,
		densityKgPerL:  1.0,
		jacketCoolTau:  0.00114,

		gasGainCoeff:   0.002,
		ventReliefCoeff: 1.5,
	}

	return t, nil
}

// --- Getters ---

func (t *Tank) ID() string        { return t.id }
func (t *Tank) Tag() string       { return t.tag }
func (t *Tank) Kind() string      { return "tank" }
func (t *Tank) CapacityL() float64 { return t.capacityL }
func (t *Tank) VolumeL() float64   { return t.volumeL }
func (t *Tank) TempC() float64     { return t.tempC }
func (t *Tank) PressureBar() float64 { return t.pressureBar }
func (t *Tank) SteamTempC() float64 { return t.steamTempC }
func (t *Tank) SteamOn() bool       { return t.steamOpening > 0 }

func (t *Tank) LevelPct() float64 {
	if t.capacityL == 0 {
		return 0
	}
	return (t.volumeL / t.capacityL) * 100.0
}

func (t *Tank) RemoveVolume(liters float64) {
	t.volumeL -= liters
	if t.volumeL < 0 {
		t.volumeL = 0
	}
}

// --- Setters (inputs do Engine) ---

func (t *Tank) SetInFlow(flowLpm, tempC float64) {
	if flowLpm < 0 {
		flowLpm = 0
	}
	t.inflowLpm = flowLpm
	t.inFlowTempC = tempC
}

func (t *Tank) SetAgitation(on bool) {
	t.agitationON = on
}

// SetSteamJacket recebe a abertura efetiva da válvula de vapor (0 a 1), de
// modo que a taxa de aquecimento da camisa acompanhe o curso do atuador.
func (t *Tank) SetSteamJacket(opening float64, supplyTempC float64) {
	if opening < 0 {
		opening = 0
	}
	if opening > 1 {
		opening = 1
	}
	t.steamOpening = opening
	t.steamSupplyTempC = supplyTempC
}

func (t *Tank) SetCleanAirInflow(flowNlpm float64) {
	if flowNlpm < 0 {
		flowNlpm = 0
	}
	t.cleanAirFlowNlpm = flowNlpm
}

// SetVentOpen recebe a abertura efetiva do respiro (0 a 1); o alívio de
// pressão é proporcional a essa abertura.
func (t *Tank) SetVentOpen(opening float64) {
	if opening < 0 {
		opening = 0
	}
	if opening > 1 {
		opening = 1
	}
	t.ventOpening = opening
}

// SetFoulingFactor aplica um fator multiplicativo ao coeficiente global de
// transferência de calor, representando a incrustação progressiva da camisa.
// Valor 1.0 corresponde à condição limpa; valores menores representam
// deposição de resíduo na parede da camisa.
func (t *Tank) SetFoulingFactor(f float64) {
	if f <= 0 {
		return
	}
	t.uaWPerk *= f
}

// SetCoolingFactor aplica um fator multiplicativo à constante de resfriamento
// da camisa, permitindo variar de corrida para corrida o tempo necessário
// para a camisa retornar à temperatura ambiente após o fechamento da válvula
// de vapor. Valor 1.0 mantém o tempo nominal; valores menores alongam o
// resfriamento.
func (t *Tank) SetCoolingFactor(f float64) {
	if f <= 0 {
		return
	}
	t.jacketCoolTau *= f
}

// --- Dinâmica ---

func (t *Tank) Tick(dtSeconds float64) {
	if dtSeconds <= 0 {
		return
	}

	// Balanço de massa
	inflowLps := t.inflowLpm / 60.0
	t.volumeL += inflowLps * dtSeconds

	if t.volumeL > t.capacityL {
		t.volumeL = t.capacityL
	}
	if t.volumeL < 0 {
		t.volumeL = 0
	}

	// Massa aproximada
	massKg := t.volumeL * t.densityKgPerL
	if massKg < 0.001 {
		massKg = 0.001
	}

	// Mistura térmica (produto): balanço de energia entre a massa já contida
	// no tanque e a massa admitida neste passo. A fração de mistura resulta da
	// razão entre as massas, e não de um coeficiente fixo — de modo que
	// encher um tanque cheio altera a temperatura menos que encher um vazio,
	// e a temperatura resultante nunca ultrapassa a da corrente de entrada.
	if t.inflowLpm > 0 && massKg > 0 {
		dMassKg := (t.inflowLpm / 60.0) * dtSeconds * t.densityKgPerL
		if dMassKg > massKg {
			dMassKg = massKg
		}
		prevMassKg := massKg - dMassKg
		t.tempC = (prevMassKg*t.tempC + dMassKg*t.inFlowTempC) / massKg
	}

	// Dinâmica térmica da camisa de vapor. O aquecimento é proporcional à
	// abertura da válvula e o resfriamento ao seu complemento, de modo que a
	// transição entre aquecer e resfriar acompanhe o curso do atuador em vez
	// de comutar de forma instantânea.
	jacketHeatTau := 0.02 // constante de tempo rápida (vapor condensa rápido)
	heatRate := jacketHeatTau * t.steamOpening * (t.steamSupplyTempC - t.steamTempC)
	coolRate := t.jacketCoolTau * (1.0 - t.steamOpening) * (t.ambientTempC - t.steamTempC)
	t.steamTempC += (heatRate + coolRate) * dtSeconds

	// Aquecimento do produto pelo vapor da camisa
	if t.steamTempC > t.tempC+1.0 {
		uaEff := t.uaWPerk
		if t.agitationON {
			uaEff *= 1.25
		}

		k := uaEff / (massKg * t.cpJPerKgK)
		t.tempC += k * (t.steamTempC - t.tempC) * dtSeconds
	}

	// Pressão
	headspaceL := t.capacityL - t.volumeL
	if headspaceL < 1 {
		headspaceL = 1
	}

	pressGain := (t.cleanAirFlowNlpm / headspaceL) * t.gasGainCoeff

	pressRelief := 0.0
	if t.ventOpening > 0 {
		pressRelief = (t.pressureBar - 1.0) * t.ventReliefCoeff * t.ventOpening
	}

	t.pressureBar += (pressGain - pressRelief) * dtSeconds
}