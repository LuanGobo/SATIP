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
}

func NewValve(id, tag string, initial ValveState) (*Valve, error) {
	if id == "" {
		return nil, fmt.Errorf("valve: id is required")
	}
	if tag == "" {
		return nil, fmt.Errorf("valve: tag is required")
	}

	v := &Valve{
		id:    id,
		tag:   tag,
		state: initial,
	}
	v.recalcFlow()
	return v, nil
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