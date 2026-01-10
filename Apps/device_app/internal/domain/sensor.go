package domain

import "fmt"

type AnalogSource interface {
	Read() float64
}

type Event struct {
	Tag     string
	Kind    string
	Value   float64
	TimeSec float64
}

type AnalogSensor struct {
	id   string
	tag  string
	kind string

	source   AnalogSource
	deadband float64

	initialized bool
	value       float64
	lastValue   float64
	timeSec     float64

	enLL, enL, enH, enHH bool
	ll, l, h, hh         float64
}

func NewAnalogSensor(
	id, tag, kind string,
	source AnalogSource,
	deadband float64,
) (*AnalogSensor, error) {

	if id == "" {
		return nil, fmt.Errorf("sensor: id is required")
	}
	if tag == "" {
		return nil, fmt.Errorf("sensor: tag is required")
	}
	if kind == "" {
		return nil, fmt.Errorf("sensor: kind is required")
	}
	if source == nil {
		return nil, fmt.Errorf("sensor: source is required")
	}
	if deadband < 0 {
		return nil, fmt.Errorf("sensor: deadband must be >= 0")
	}

	return &AnalogSensor{
		id:       id,
		tag:      tag,
		kind:     kind,
		source:   source,
		deadband: deadband,
	}, nil
}

func (s *AnalogSensor) ConfigureAlarms(
	enLL bool, ll float64,
	enL bool, l float64,
	enH bool, h float64,
	enHH bool, hh float64,
) {
	s.enLL, s.ll = enLL, ll
	s.enL, s.l = enL, l
	s.enH, s.h = enH, h
	s.enHH, s.hh = enHH, hh
}

func (s *AnalogSensor) Tick(dtSeconds float64) []Event {
	s.timeSec += dtSeconds

	raw := s.source.Read()

	if !s.initialized {
		s.initialized = true
		s.value = raw
		s.lastValue = raw
		return nil
	}

	if abs(raw-s.value) >= s.deadband {
		s.lastValue = s.value
		s.value = raw
	}

	return s.evalAlarms()
}

func (s *AnalogSensor) evalAlarms() []Event {
	var events []Event
	v := s.value

	if s.enLL && v <= s.ll {
		events = append(events, s.makeEvent("alarm_ll", v))
	}
	if s.enL && v <= s.l {
		events = append(events, s.makeEvent("alarm_l", v))
	}
	if s.enH && v >= s.h {
		events = append(events, s.makeEvent("alarm_h", v))
	}
	if s.enHH && v >= s.hh {
		events = append(events, s.makeEvent("alarm_hh", v))
	}

	return events
}

func (s *AnalogSensor) makeEvent(kind string, v float64) Event {
	return Event{
		Tag:     s.tag,
		Kind:    kind,
		Value:   v,
		TimeSec: s.timeSec,
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}