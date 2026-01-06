package app

import (
	"fmt"
	"strings"

	"device_app/internal/domain"
)

func ResolveAnalogSource(m *Model, src string) (domain.AnalogSource, error) {
	parts := strings.Split(src, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid source '%s' (expected tank.<id>.<field>)", src)
	}

	if parts[0] != "tank" {
		return nil, fmt.Errorf("unsupported source kind '%s'", parts[0])
	}

	tk, ok := m.Tanks[parts[1]]
	if !ok {
		return nil, fmt.Errorf("tank '%s' not found", parts[1])
	}

	switch parts[2] {
	case "temp":
		return TankTempSource{tk}, nil
	case "level_pct":
		return TankLevelPctSource{tk}, nil
	case "pressure_bar":
		return TankPressureSource{tk}, nil
	default:
		return nil, fmt.Errorf("unsupported tank field '%s'", parts[2])
	}
}

// --- Adapters ---

type TankTempSource struct{ Tank *domain.Tank }
func (s TankTempSource) Read() float64 { return s.Tank.TempC() }

type TankLevelPctSource struct{ Tank *domain.Tank }
func (s TankLevelPctSource) Read() float64 { return s.Tank.LevelPct() }

type TankPressureSource struct{ Tank *domain.Tank }
func (s TankPressureSource) Read() float64 { return s.Tank.PressureBar() }