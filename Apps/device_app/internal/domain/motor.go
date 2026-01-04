package domain

import (
	"fmt"
)

type MotorState int

const (
	Stopped MotorState = iota
	RunningForward
	RunningReverse
)

func (m MotorState) String() string {
	switch m {
	case RunningForward:
		return "running in forward"
	case RunningReverse:
		return "running in reverse"
	case Stopped:
		return "stopped"
	default:
		return "unknown"
	}
}

type Motor struct {
	id         string
	tag        string
	state      MotorState
	hasLoad    bool
	rpm        float64
	isActive   bool
	hasReverse bool
}

func NewMotor(id string, tag string, initial MotorState, hasReverse bool) (*Motor, error) {
	if id == "" {
		return nil, fmt.Errorf("motor: id is required")
	}
	m := &Motor{
		id:         id,
		tag:        tag,
		state:      initial,
		hasReverse: hasReverse,
	}
	m.recalcActive()
	return m, nil
}

func (m *Motor) ID() string        { return m.id }
func (m *Motor) Tag() string       { return m.tag }
func (m *Motor) Kind() string      { return "motor" }
func (m *Motor) State() MotorState { return m.state }

func (m *Motor) IsRunning() bool {
	return m.state == RunningForward || m.state == RunningReverse
}

func (m *Motor) IsActive() bool { return m.isActive }
func (m *Motor) HasLoad() bool  { return m.hasLoad }
func (m *Motor) RPM() float64   { return m.rpm }

func (m *Motor) StartForward() {
	m.state = RunningForward
	m.recalcActive()
}

func (m *Motor) StartReverse() {
	if m.hasReverse {
		m.state = RunningReverse
		m.recalcActive()
	}
}

func (m *Motor) Stop() {
	m.state = Stopped
	m.recalcActive()
}

func (m *Motor) SetLoad(has bool) {
	m.hasLoad = has
	m.recalcActive()
}

func (m *Motor) recalcActive() {
	m.isActive = (m.state == RunningForward || m.state == RunningReverse) && m.hasLoad

	if !m.isActive {
		m.rpm = 0
	} else {
		m.rpm = 150.5
	}
}