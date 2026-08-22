package domain

import "fmt"

type ValveState int

const (
	ValveClosed ValveState = iota
	ValveOpen
)

func (s ValveState) String() string {
	switch s {
	case ValveOpen:
		return "open"
	case ValveClosed:
		return "closed"
	default:
		return "unknown"
	}
}

type Valve struct {
	id    string
	tag   string
	state ValveState

	inflowPresent bool
	flowActive    bool

	// Curso do atuador: a válvula não abre instantaneamente. position varia
	// de 0 (fechada) a 1 (totalmente aberta), avançando a cada Tick em
	// direção ao estado comandado, o que produz a transição suave de vazão
	// característica de um atuador pneumático real.
	position       float64
	strokeSeconds  float64

	// stuckMinimum representa obstrução mecânica no atuador: a válvula não
	// consegue fechar abaixo desta abertura, ainda que comandada a fechar.
	stuckMinimum float64
}

// SetStuckMinimum simula obstrução mecânica no atuador, impedindo o
// fechamento completo da válvula. Valor 0 corresponde a atuador íntegro.
func (v *Valve) SetStuckMinimum(f float64) {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	v.stuckMinimum = f
}

// IsStuck informa se o atuador está obstruído.
func (v *Valve) IsStuck() bool { return v.stuckMinimum > 0 }

func NewValve(id, tag string, initial ValveState) (*Valve, error) {
	if id == "" {
		return nil, fmt.Errorf("valve: id is required")
	}
	if tag == "" {
		return nil, fmt.Errorf("valve: tag is required")
	}

	v := &Valve{
		id:            id,
		tag:           tag,
		state:         initial,
		strokeSeconds: 12.0,
	}
	if initial == ValveOpen {
		v.position = 1.0
	}
	v.recalcFlow()
	return v, nil
}

// SetStrokeSeconds ajusta o tempo de curso completo do atuador.
func (v *Valve) SetStrokeSeconds(s float64) {
	if s > 0 {
		v.strokeSeconds = s
	}
}

// Position devolve a abertura efetiva da válvula (0 a 1).
func (v *Valve) Position() float64 { return v.position }

// Tick avança o curso do atuador em direção ao estado comandado.
func (v *Valve) Tick(dtSeconds float64) {
	if dtSeconds <= 0 || v.strokeSeconds <= 0 {
		return
	}

	target := 0.0
	if v.state == ValveOpen {
		target = 1.0
	}

	stepMax := dtSeconds / v.strokeSeconds
	delta := target - v.position

	switch {
	case delta > stepMax:
		v.position += stepMax
	case delta < -stepMax:
		v.position -= stepMax
	default:
		v.position = target
	}

	if v.position < v.stuckMinimum {
		v.position = v.stuckMinimum
	}
	if v.position < 0 {
		v.position = 0
	}
	if v.position > 1 {
		v.position = 1
	}
}

func (v *Valve) ID() string   { return v.id }
func (v *Valve) Tag() string  { return v.tag }
func (v *Valve) Kind() string { return "valve" }

func (v *Valve) IsOpen() bool {
	return v.state == ValveOpen
}

func (v *Valve) Open() {
	v.state = ValveOpen
	v.recalcFlow()
}

func (v *Valve) Close() {
	v.state = ValveClosed
	v.recalcFlow()
}

func (v *Valve) SetInflowPresent(has bool) {
	v.inflowPresent = has
	v.recalcFlow()
}

func (v *Valve) HasFlow() bool {
	return v.flowActive
}

func (v *Valve) recalcFlow() {
	v.flowActive = v.IsOpen() && v.inflowPresent
}