package domain

import(
	"fmt"
)

type Tank struct {
	id 	string
	tag string

	tempC float64

	//capacidade do tanque
	capacityL 	float64
	volumeL 	float64

	//Entradas no tanque
	inflowLpm 	float64 // L/min
	inFlowTempC 	float64 // temperatudo do produto de entrada °C
	
	//Agitação Ligada
	agitationON bool

	//Parametros da entrada de Vapor
	steamON 	bool
	steamTempC float64 //Temperatura do vapor

	//Parâmetros térmicos (simplificados)
	uaWPerk			float64	// W/K (força de troca térmica)
	cpJPerkgk		float64	// J/kg·K
	densitykgPerL	float64	// kg/L (aprox água ~1.0)
}

func Newtank(
	id, tag string,
	capacityL, initialValumeL, initialTempC float64,
)	(*Tank, error) {
	if id == ""{
		return nil, fmt.Errorf("tank: id is required")
	}
	if capacityL <= 0 {
		return nil, fmt.Errorf("tank: capacityL must be > 0")
	}
	if initialValumeL < 0 || initialValumeL > capacityL {
		return nil, fmt.Errorf("tank: initialVolumeL must be between 0 and capacityL")
	}
	
	t := &Tank{
		id:			id,
		tag: 		tag,
		capacityL: 	capacityL,
		volumeL: 	volumeL,
		tempC: 		initialTempC,

		//Setpoints razóaveis para o simulador
		inFlowTempC: 	initialTempC,
		uaWPerk:		900.0,   // valor placeholder: ajuste/calibre depois
		cpJPerKgK:   	4180.0,  // água ~4180 J/kgK
		densityKgPerL: 	1.0,   // água ~1.0 kg/L
	}

	return t, nil
}

func (t *Tank) ID() string {return t.id}
func (t *Tank) Tag() string {return t.tag}
func (t *Tank) Kind() string {return "tank"}

func (t *Tank) CapacityL() float64 {return t.capacityL}
func (t *Tank) VolumeL() float64 {return t.volumeL}
func (t *Tank) TempC() float64 {return t.tempC}

func (t *Tank) LevelPct() float64 {
	if t.capacityL == 0{
		return 0
	}
	return (t.volumeL / t.capacityL) * 100.0
}

func (t *Tank) SetInFlow(inflowLpm, inFlowTempC float64){
	if inflowLpm < 0 {
		inflowLpm = 0
	}

	t.inflowLpm = 	inflowLpm
	t.inFlowTempC = inFlowTempC
}

func (t *Tank) SetAgitation(on bool) {
	t.agitationON = on
}

func (t *Tank) SetSteamJacket (on bool, steamTempC float64) {
	t.steamON = on
	t.steamTempC = steamTempC
}

func (t *Tank) SetThermalParams(uaWPerk, cpJPerKgK, densityKgPerL float64) {
	if uaWPerk > 0 {
		t.uaWPerk = uaWPerk
	}
	if cpJPerKgK > 0 {
		t.cpJPerKgK = cpJPerKgK
	}
	if densityKgPerL > 0 {
		t.densityKgPerL = densityKgPerL
	}
}

func (t *Tank) Tick(dtSeconds float64) {
	if dtSeconds <= 0{
		return
	}
	

	//Balanço de massa
	inflowLps := t.inflowLpm / 60.0
	t.volumeL += inflowLps * dtSeconds

	if t.volumeL > t.capacityL {
		t.volumeL = t.capacityL
	}

	if t.volumeL < 0 {
		t.volumeL = 0
	}

	//Massa aproximada no tanque
	massKg := t.volumeL * t.densityKgPerL
	if massKg < 0.001 {
		massKg = 0.001
	}


	// Mistura Térmica por entrada de produto
	if t.inflowLpm > 0 {
		alpha := 0.05
		if t.agitationON{
			alpha = 0.15
		}
		t.tempC = t.tempC + alpha*(t.inFlowTempC-t.tempC)
	}

	//Aquecimento por vapor na camisa
	// Modelo lumped: dT/dt = (UA/(m*Cp)) * (Tsteam - Ttank)
	if t.steamON {
		uaEff := t.uaWPerk
		if t.agitationON{
			uaEff *= 1.25
		}

		k := uaEff / (massKg * t.cpJPerKgK)
		t.tempC = tempC + (k + (t.steamTempC - t.tempC) * dtSeconds)
	}
	
}