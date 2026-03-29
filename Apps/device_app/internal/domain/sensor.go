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

	// Alarm state tracking (for edge detection)
	alarmActiveLL bool
	alarmActiveL  bool
	alarmActiveH  bool
	alarmActiveHH bool
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

func (s *AnalogSensor) Value() float64    { return s.value }
func (s *AnalogSensor) ID() string        { return s.id }
func (s *AnalogSensor) Tag() string       { return s.tag }
func (s *AnalogSensor) Kind() string      { return s.kind }
func (s *AnalogSensor) Deadband() float64 { return s.deadband }

func (s *AnalogSensor) AlarmActiveLL() bool { return s.alarmActiveLL }
func (s *AnalogSensor) AlarmActiveL() bool  { return s.alarmActiveL }
func (s *AnalogSensor) AlarmActiveH() bool  { return s.alarmActiveH }
func (s *AnalogSensor) AlarmActiveHH() bool { return s.alarmActiveHH }
func (s *AnalogSensor) AlarmEnabledLL() bool { return s.enLL }
func (s *AnalogSensor) AlarmEnabledL() bool  { return s.enL }
func (s *AnalogSensor) AlarmEnabledH() bool  { return s.enH }
func (s *AnalogSensor) AlarmEnabledHH() bool { return s.enHH }

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

// evalAlarms fires alarm events only on TRANSITIONS (rising edge).
// When the alarm condition becomes true, it fires once.
// It does NOT fire again until the condition clears and re-occurs.
func (s *AnalogSensor) evalAlarms() []Event {
	var events []Event
	v := s.value

	// LL alarm
	if s.enLL {
		wasActive := s.alarmActiveLL
		s.alarmActiveLL = v <= s.ll
		if s.alarmActiveLL && !wasActive {
			events = append(events, s.makeEvent("alarm_ll", v))
		}
	}

	// L alarm
	if s.enL {
		wasActive := s.alarmActiveL
		s.alarmActiveL = v <= s.l
		if s.alarmActiveL && !wasActive {
			events = append(events, s.makeEvent("alarm_l", v))
		}
	}

	// H alarm
	if s.enH {
		wasActive := s.alarmActiveH
		s.alarmActiveH = v >= s.h
		if s.alarmActiveH && !wasActive {
			events = append(events, s.makeEvent("alarm_h", v))
		}
	}

	// HH alarm
	if s.enHH {
		wasActive := s.alarmActiveHH
		s.alarmActiveHH = v >= s.hh
		if s.alarmActiveHH && !wasActive {
			events = append(events, s.makeEvent("alarm_hh", v))
		}
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
