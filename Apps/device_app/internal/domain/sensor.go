package domain

import (
	"fmt"
)

type AlarmLevel int

const (
	AlarmNone AlarmLevel = iota
	AlarmLL
	AlarmL
	AlarmH
	AlarmHH
)

func (a AlarmLevel) String() string{
	switch a{
	case AlarmLL:
		return "LL"
	case AlarmL:
		return "L"
	case AlarmH:
		return "H"
	case AlarmHH:
		return "HH"
	case AlarmNone:
		return "NONE"
	default:
		return "unknown"
	}
}

type AnalogSource interface{
	Read() float64
}

type Event struct {
	SourceID 	string
	SourceTag 	string
	EventType 	string
	AlarmLevel 	AlarmLevel
	Value 		float64
	Message 	string
}

type AnalogSensor struct {
	id 			string
	tag 		string
	kind 		string

	source 		AnalogSource

	enabledLL	bool
	enabledL	bool
	enabledH	bool
	enabledHH	bool

	limitLL		float64
	limitL		float64
	limitH		float64
	limitHH		float64

	deadband	float64

	lastValue	float64
	alarmLevel 	AlarmLevel
}

func NewAnalogSensor(id, tag, kind string, source, AnalogSource, deadband float64) (*AnalogSensor, error) {
	if id == "" {
		return nil, fmt.Errorf("sensor: id is required")
	}
	if source == nil {
		return nil, fmt.Errorf("sensor: source is required")	
	}
	if deadband < 0 {
		deadband = 0
	}

	s := &AnalogSensor{
		id:	id,
		tag:	tag,
		kind: kind,
		source: source,
		deadband:deadband,
		alarmLevel:	AlarmNone
	}
	s.lastValue = source.Read()
	return s,nil
}

func (s *AnalogSensor) ID() string {return s.id}
func (s *AnalogSensor) Tag() string {return s.tag}
func (s *AnalogSensor) Kind() string {return s.kind}
func (s *AnalogSensor) Value() float64 {return s.lastValue}
func (s *AnalogSensor) AlarmLevel() AlarmLevel {return s.alarmLevel}

func (s *AnalogSensor) ConfigureAlarms(	
	enabledLL bool, limitLL float64,
	enabledL bool, 	limitL float64,
	enabledH bool, 	limitH float64,
	enabledHH bool, limitHH float64,
	) {
		s.enabledLL, s.limitLL = enabledLL, limitLL
		s.enabledL, s.limitLL = enabledL, limitLL
		s.enabledH, s.limitLL = enabledH, limitLL
		s.enabledHH, s.limitLL = enabledHH, limitLL
}

func (s *AnalogSensor) Tick(dtSeconds float64) (events []Event) {
	_ = dtSeconds

	val := s.source.Read()
	s.lastValue = val

	newLevel = s.evalAlarm(val)

	if s.alarmLevel != s.alarmLevel {
		if s.alarmLevel != AlarmNone {
			events = append(events, Event{
				SourceID:   s.id,
				SourceTag:  s.tag,
				EventType:  "ALARM_EXIT",
				AlarmLevel: s.alarmLevel,
				Value:      val,
				Message:    "alarm exited",
			})
		}
		if newLevel != AlarmNone{
			events = append(events, Event{
				SourceID:   s.id,
				SourceTag:  s.tag,
				EventType:  "ALARM_ENTER",
				AlarmLevel: s.alarmLevel,
				Value:      val,
				Message:    "alarm entered",
			})
		}

		s.alarmLevel = newLevel
	}
	return events
}

func (s *AnalogSensor) evalAlarm(val float64) AlarmLevel {
	switch s.alarmLevel{
	case AlarmHH:
		if!(s.enabledHH && val > s.limitHH-s.deadband){
			break
		}
		return AlarmHH
	case AlarmH:
		if!(s.enabledH && val > s.limitH-s.deadband){
			break
		}
		return AlarmH
	case AlarmL:
		if!(s.enabledL && val > s.limitL+s.deadband){
			break
		}
		return AlarmL
	case AlarmLL:
		if!(s.enabledLL && val > s.limitLL+s.deadband){
			break
		}
		return AlarmLL
	}

	if s.enabledHH && val >= s.limitHH {
		return AlarmHH
	}
	if s.enabledH && val >= s.limitH {
		return AlarmH
	}
	if s.enabledL && val >= s.limitL {
		return AlarmL
	}
	if s.enabledLL && val >= s.limitLL {
		return AlarmLL
	}
	return AlarmNone
}

