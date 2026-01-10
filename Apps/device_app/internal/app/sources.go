package app

import (
	"fmt"
	"strings"

	"device_app/internal/domain"
)

type analogSourceFunc func() float64

func (f analogSourceFunc) Read() float64 { return f() }

// ResolveAnalogSource aceita strings como:
//   tank.tk-01.temp
//   tank.tk-01.level_pct
//   tank.tk-01.pressure_bar
//   tank.tk-01.steam_temp
func ResolveAnalogSource(m *Model, expr string) (domain.AnalogSource, error) {
	parts := strings.Split(strings.TrimSpace(expr), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("source: invalid format '%s' (expected 'tank.<id>.<field>')", expr)
	}

	kind := parts[0]
	id := parts[1]
	field := parts[2]

	switch kind {
	case "tank":
		tk := m.Tanks[id]
		if tk == nil {
			return nil, fmt.Errorf("source: tank '%s' not found", id)
		}

		switch field {
		case "temp":
			return analogSourceFunc(func() float64 { return tk.TempC() }), nil
		case "level_pct":
			return analogSourceFunc(func() float64 { return tk.LevelPct() }), nil
		case "pressure_bar":
			return analogSourceFunc(func() float64 { return tk.PressureBar() }), nil
		case "steam_temp":
			return analogSourceFunc(func() float64 { return tk.SteamTempC() }), nil
		default:
			return nil, fmt.Errorf("source: unsupported tank field '%s'", field)
		}

	default:
		return nil, fmt.Errorf("source: unsupported kind '%s'", kind)
	}
}