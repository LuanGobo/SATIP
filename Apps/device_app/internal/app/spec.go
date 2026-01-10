package app

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type PlantSpec struct {
	Plant PlantInfo					`yaml:"plant"`
	Simulation SimulationSpec		`yaml:"simulation"`
	Equipment EquipmentSpec			`yaml:"equipment"`
	Connections []ConnectionSpec	`yaml:"connections"`
}

type PlantInfo struct {
	ID	string	`yaml:"id"`
	Name string	`yaml:"name"`
}

type SimulationSpec struct{
	DTSeconds 		float64	`yaml:"dt_seconds"`
	DurationSeconds float64	`yaml:"duration_seconds"`
}

type EquipmentSpec struct {
	Tanks 	[]TankSpec		`yaml:"tanks"`
	Valves	[]ValveSpec		`yaml:"valves"`
	Motors	[]MotorSpec		`yaml:"motors"`
	Sensors	[]SensorSpec 	`yaml:"sensors"`
}

type TankSpec struct {
	ID	string `yaml:"id"`
	Tag string `yaml:"tag"`
	CapacityL float64 `yaml:"capacity_l"`
	InitialVolumeL	float64 `yaml:"initial_volume_l"`
	InitialTempC float64 `yaml:"initial_temp_c"`
}

type ValveSpec struct {
	ID string `yaml:"id"`
	Tag string `yaml:"tag"`
	Service string `yaml:"service"`
	InitialOpen bool `yaml:"initial_open"`
}

type MotorSpec struct {
	ID string  `yaml:"id"`
	Tag string `yaml:"tag"`
	InitialState string `yaml:"initial_state"`
	HasReverse bool `yaml:"has_reverse"`
}

type SensorSpec struct {
	ID string  `yaml:"id"`
	Tag string `yaml:"tag"`
	Kind string `yaml:"kind"`
	Source string `yaml:"source"`
	Deadband float64 `yaml:"deadband"`
	Alarms map[string]float64 `yaml:"alarms"`
}

//Connections
type ConnectionSpec struct{
	ID string   `yaml:"id"`
	Type string `yaml:"type"`
	Path []string `yaml:"path"`

	Source ConnectionSourceSpec `yaml:"source"`
	Target ConnectionTargetSpec `yaml:"target"`

	Discharge *DischargeSpec `yaml:"discharge,omitempty"`
}

type ConnectionTargetSpec struct {
	EquipmentID string	`yaml:"equipment_id"`
	Port		string	`yaml:"port"`
}

type DischargeSpec struct {
	To string `yaml:"to"`
}

type ConnectionSourceSpec struct {
	// product_inlet / cip_inlet
	ProductName string  `yaml:"product_name"`
	FlowLpm     float64 `yaml:"flow_lpm"`
	TempC       float64 `yaml:"temp_c"`
	Chemical    string  `yaml:"chemical"`

	// clean_air_inlet
	FlowNlpm    float64 `yaml:"flow_nlpm"`
	PressureBar float64 `yaml:"pressure_bar"`

	// steam_jacket
	// reutiliza TempC (temp_c)

	// transfer_outlet
	FromTankID      string  `yaml:"from_tank_id"`
	NominalFlowLpm  float64 `yaml:"nominal_flow_lpm"`

	// agitation_link
	MotorID            string  `yaml:"motor_id"`
	RequiresVolumeGtL  float64 `yaml:"requires_volume_gt_l"`

	// vent_outlet
	Mode              string  `yaml:"mode"`
	TargetPressureBar float64 `yaml:"target_pressure_bar"`
}

func LoadPlantSpec(path string) (*PlantSpec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}

	var spec PlantSpec
	if err := yaml.Unmarshal(b, &spec); err != nil {
		return nil, fmt.Errorf("yaml unmarshal: %w", err)
	}

	if err := spec.Validate(); err != nil{
		return nil, err
	}

	return &spec, nil
}

func (s *PlantSpec) Validate() error{
	if strings.TrimSpace(s.Plant.ID) == "" {
		return fmt.Errorf("spec: plant.id is required")
	}

	if s.Simulation.DTSeconds <= 0{
		return fmt.Errorf("spec: simulation.dt_seconds must be > 0")
	}

	if s.Simulation.DurationSeconds <= 0 {
		return fmt.Errorf("spec: simulation.duration_seconds must be > 0")
	}

	for i, t := range s.Equipment.Tanks {
		if t.ID == "" {
			return fmt.Errorf("spec: equipment.tanks[%d].id is required", i)
		}
		if t.CapacityL <= 0 {
			return fmt.Errorf("spec: equipment.tanks[%d].capacity_l must be > 0", i)
		}
	}

	for i, v := range s.Equipment.Valves {
		if v.ID == "" {
			return fmt.Errorf("spec: equipment.valves[%d].id is required", i)
		}
		if v.Tag == "" {
			return fmt.Errorf("spec: equipment.valves[%d].tag is required", i)
		}
	}

	for i, m := range s.Equipment.Motors {
		if m.ID == "" {
			return fmt.Errorf("spec: equipment.motors[%d].id is required", i)
		}
	}

	for i, sn := range s.Equipment.Sensors {
		if sn.ID == "" {
			return fmt.Errorf("spec: equipment.sensors[%d].id is required", i)
		}
		if sn.Source == "" {
			return fmt.Errorf("spec: equipment.sensors[%d].source is required", i)
		}
		if sn.Deadband < 0 {
			return fmt.Errorf("spec: equipment.sensors[%d].deadband must be >= 0", i)
		}
	}

	for i, c := range s.Connections {
		if c.ID == "" {
			return fmt.Errorf("spec: connections[%d].id is required", i)
		}
		if c.Type == "" {
			return fmt.Errorf("spec: connections[%d].type is required", i)
		}
		if len(c.Path) == 0 && c.Type != "agitation_link" {
			return fmt.Errorf("spec: connections[%d].path must have at least 1 element", i)
		}
		if c.Target.EquipmentID == "" {
			return fmt.Errorf("spec: connections[%d].target.equipment_id is required", i)
		}
		if c.Target.Port == "" {
			return fmt.Errorf("spec: connections[%d].target.port is required", i)
		}
		switch c.Type {
		case "product_inlet", "cip_inlet", "clean_air_inlet", "steam_jacket",
			"transfer_outlet", "agitation_link", "vent_outlet":
			// ok
		default:
			return fmt.Errorf("spec: connections[%d].type '%s' is not supported", i, c.Type)
		}
	}

	return nil
}