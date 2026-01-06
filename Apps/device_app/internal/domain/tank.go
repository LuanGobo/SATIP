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
	ventOpen         bool

	// Agitação
	agitationON bool

	// Vapor
	steamON    bool
	steamTempC float64

	// Parâmetros térmicos
	uaWPerk        float64
	cpJPerKgK      float64
	densityKgPerL  float64

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

		uaWPerk:        900.0,
		cpJPerKgK:      4180.0,
		densityKgPerL:  1.0,

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

func (t *Tank) LevelPct() float64 {
	if t.capacityL == 0 {
		return 0
	}
	return (t.volumeL / t.capacityL) * 100.0
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

func (t *Tank) SetSteamJacket(on bool, steamTempC float64) {
	t.steamON = on
	t.steamTempC = steamTempC
}

func (t *Tank) SetCleanAirInflow(flowNlpm float64) {
	if flowNlpm < 0 {
		flowNlpm = 0
	}
	t.cleanAirFlowNlpm = flowNlpm
}

func (t *Tank) SetVentOpen(open bool) {
	t.ventOpen = open
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

	// Mistura térmica (produto)
	if t.inflowLpm > 0 {
		alpha := 0.05
		if t.agitationON {
			alpha = 0.15
		}
		t.tempC += alpha * (t.inFlowTempC - t.tempC)
	}

	// Aquecimento por vapor
	if t.steamON {
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
	if t.ventOpen {
		pressRelief = (t.pressureBar - 1.0) * t.ventReliefCoeff
	}

	t.pressureBar += (pressGain - pressRelief) * dtSeconds
}