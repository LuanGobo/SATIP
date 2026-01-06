package app

import (
	"fmt"
	"strings"

	"device_app/internal/domain"
)

type Model struct {
	DTSeconds       float64
	DurationSeconds float64

	Tanks   map[string]*domain.Tank
	Valves  map[string]*domain.Valve
	Motors  map[string]*domain.Motor
	Sensors map[string]*domain.AnalogSensor

	Connections []ConnectionSpec
}

func BuildModel(spec *PlantSpec) (*Model, error) {
	if spec == nil {
		return nil, fmt.Errorf("builder: spec is nil")
	}

	if err := validateNoDuplicateIDs(spec); err != nil {
		return nil, err
	}

	if err := validateReferences(spec); err != nil {
		return nil, err
	}

	m := &Model{
		DTSeconds:       spec.Simulation.DTSeconds,
		DurationSeconds: spec.Simulation.DurationSeconds,
		Tanks:           map[string]*domain.Tank{},
		Valves:          map[string]*domain.Valve{},
		Motors:          map[string]*domain.Motor{},
		Sensors:         map[string]*domain.AnalogSensor{},
		Connections:     spec.Connections,
	}

	// Instancia Tanques
	for _, t := range spec.Equipment.Tanks {
		tk, err := domain.NewTank(t.ID, t.Tag, t.CapacityL, t.InitialVolumeL, t.InitialTempC)
		if err != nil {
			return nil, fmt.Errorf("builder: tank %s: %w", t.ID, err)
		}
		m.Tanks[t.ID] = tk
	}

	// Instancia Valvulas
	for _, v := range spec.Equipment.Valves {
		initial := domain.ValveClosed
		if v.InitialOpen {
			initial = domain.ValveOpen
		}

		vv, err := domain.NewValve(v.ID, v.Tag, initial)
		if err != nil {
			return nil, fmt.Errorf("builder: valve %s: %w", v.ID, err)
		}
		m.Valves[v.ID] = vv
	}

	// Instancia Motores
	for _, mm := range spec.Equipment.Motors {
		initial, err := parseMotorState(mm.InitialState)
		if err != nil {
			return nil, fmt.Errorf("builder: motor %s: %w", mm.ID, err)
		}

		motor, err := domain.NewMotor(mm.ID, mm.Tag, initial, mm.HasReverse)
		if err != nil {
			return nil, fmt.Errorf("builder: motor %s: %w", mm.ID, err)
		}
		m.Motors[mm.ID] = motor
	}

	// Instancia Sensores
	for _, ss := range spec.Equipment.Sensors {
		src, err := ResolveAnalogSource(m, ss.Source)
		if err != nil {
			return nil, fmt.Errorf("builder: sensor %s source '%s': %w", ss.ID, ss.Source, err)
		}

		sensor, err := domain.NewAnalogSensor(ss.ID, ss.Tag, ss.Kind, src, ss.Deadband)
		if err != nil {
			return nil, fmt.Errorf("builder: sensor %s: %w", ss.ID, err)
		}

		// Alarmes do YAML: ll, l, h, hh (ausente => desabilitado)
		enableLL, ll := getAlarm(ss.Alarms, "ll")
		enableL, l := getAlarm(ss.Alarms, "l")
		enableH, h := getAlarm(ss.Alarms, "h")
		enableHH, hh := getAlarm(ss.Alarms, "hh")

		sensor.ConfigureAlarms(enableLL, ll, enableL, l, enableH, h, enableHH, hh)

		m.Sensors[ss.ID] = sensor
	}

	return m, nil
}

// ---------------------------
// Validações
// ---------------------------

func validateNoDuplicateIDs(spec *PlantSpec) error {
	seen := map[string]string{}

	add := func(section, id string) error {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("builder: empty id in %s", section)
		}
		if prev, ok := seen[id]; ok {
			return fmt.Errorf("builder: duplicated id '%s' in %s and %s", id, prev, section)
		}
		seen[id] = section
		return nil
	}

	for i, t := range spec.Equipment.Tanks {
		if err := add(fmt.Sprintf("equipment.tanks[%d]", i), t.ID); err != nil {
			return err
		}
	}
	for i, v := range spec.Equipment.Valves {
		if err := add(fmt.Sprintf("equipment.valves[%d]", i), v.ID); err != nil {
			return err
		}
	}
	for i, m := range spec.Equipment.Motors {
		if err := add(fmt.Sprintf("equipment.motors[%d]", i), m.ID); err != nil {
			return err
		}
	}
	for i, s := range spec.Equipment.Sensors {
		if err := add(fmt.Sprintf("equipment.sensors[%d]", i), s.ID); err != nil {
			return err
		}
	}
	for i, c := range spec.Connections {
		if err := add(fmt.Sprintf("connections[%d]", i), c.ID); err != nil {
			return err
		}
	}

	return nil
}

func validateReferences(spec *PlantSpec) error {
	tanks := map[string]struct{}{}
	valves := map[string]struct{}{}
	motors := map[string]struct{}{}

	for _, t := range spec.Equipment.Tanks {
		tanks[t.ID] = struct{}{}
	}
	for _, v := range spec.Equipment.Valves {
		valves[v.ID] = struct{}{}
	}
	for _, m := range spec.Equipment.Motors {
		motors[m.ID] = struct{}{}
	}

	for i, c := range spec.Connections {
		// target
		if _, ok := tanks[c.Target.EquipmentID]; !ok {
			return fmt.Errorf("builder: connections[%d] target tank '%s' not found", i, c.Target.EquipmentID)
		}

		// path
		for j, pid := range c.Path {
			if _, ok := valves[pid]; ok {
				continue
			}
			if _, ok := motors[pid]; ok {
				continue
			}
			return fmt.Errorf("builder: connections[%d].path[%d] '%s' not found in valves or motors", i, j, pid)
		}

		// validação por tipo
		switch c.Type {
		case "agitation_link":
			if strings.TrimSpace(c.Source.MotorID) == "" {
				return fmt.Errorf("builder: connections[%d] agitation_link requires source.motor_id", i)
			}
			if _, ok := motors[c.Source.MotorID]; !ok {
				return fmt.Errorf("builder: connections[%d] agitation_link motor '%s' not found", i, c.Source.MotorID)
			}

		case "transfer_outlet":
			if strings.TrimSpace(c.Source.FromTankID) == "" {
				return fmt.Errorf("builder: connections[%d] transfer_outlet requires source.from_tank_id", i)
			}
			if _, ok := tanks[c.Source.FromTankID]; !ok {
				return fmt.Errorf("builder: connections[%d] transfer_outlet from_tank '%s' not found", i, c.Source.FromTankID)
			}
		}
	}

	return nil
}

// ---------------------------
// Helpers
// ---------------------------

func parseMotorState(s string) (domain.MotorState, error) {
	ss := strings.TrimSpace(strings.ToLower(s))
	if ss == "" || ss == "stopped" {
		return domain.Stopped, nil
	}
	switch ss {
	case "running_forward", "forward":
		return domain.RunningForward, nil
	case "running_reverse", "reverse":
		return domain.RunningReverse, nil
	default:
		return domain.Stopped, fmt.Errorf("invalid initial_state '%s' (use stopped/forward/reverse)", s)
	}
}

func getAlarm(m map[string]float64, key string) (enabled bool, value float64) {
	if m == nil {
		return false, 0
	}
	v, ok := m[key]
	if !ok {
		return false, 0
	}
	return true, v
}