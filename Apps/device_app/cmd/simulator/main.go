// Apps/device_app/cmd/simulator/main.go
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"device_app/internal/app"
)

// bool01 escreve 1/0 para facilitar gráficos (Excel/PowerBI).
func bool01(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func main() {
	// CLI
	cfgPath := flag.String("config", "cmd/simulator/plant.yaml", "path to plant YAML")
	outDir := flag.String("out", "out", "output directory (CSV files)")
	flag.Parse()

	// 1) Carrega spec
	spec, err := app.LoadPlantSpec(*cfgPath)
	if err != nil {
		log.Fatalf("load spec: %v", err)
	}

	// 2) Build model
	model, err := app.BuildModel(spec)
	if err != nil {
		log.Fatalf("build model: %v", err)
	}

	// 3) Engine
	engine, err := app.NewEngine(model)
	if err != nil {
		log.Fatalf("new engine: %v", err)
	}

	// 4) Controller (sequência por condição)
	//
	// IMPORTANTE: É AQUI que fica a "lógica de comandos" da simulação.
	// Você NÃO deve programar a lógica no Engine nem no Domain.
	// O Controller decide (por condição/tempo/estado) quais válvulas/motores ligar/desligar.
	//
	// O arquivo onde você implementa a lógica é:
	//   Apps/device_app/internal/app/controller.go
	//
	// Se quiser múltiplos cenários, crie outros controllers (ou parametrizações).
	controller := app.NewController("tk-01")

	// 5) Output dir
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		log.Fatalf("mkdir out: %v", err)
	}

	// 6) CSV timeseries
	tsPath := filepath.Join(*outDir, "timeseries.csv")
	tsFile, err := os.Create(tsPath)
	if err != nil {
		log.Fatalf("create timeseries.csv: %v", err)
	}
	defer tsFile.Close()

	w := csv.NewWriter(tsFile)
	defer w.Flush()

	// 7) CSV events (sensores + controller)
	evPath := filepath.Join(*outDir, "events.csv")
	evFile, err := os.Create(evPath)
	if err != nil {
		log.Fatalf("create events.csv: %v", err)
	}
	defer evFile.Close()

	ew := csv.NewWriter(evFile)
	defer ew.Flush()

	// 8) Ordem determinística de IDs (colunas)
	tankIDs := make([]string, 0, len(model.Tanks))
	for id := range model.Tanks {
		tankIDs = append(tankIDs, id)
	}
	sort.Strings(tankIDs)

	valveIDs := make([]string, 0, len(model.Valves))
	for id := range model.Valves {
		valveIDs = append(valveIDs, id)
	}
	sort.Strings(valveIDs)

	motorIDs := make([]string, 0, len(model.Motors))
	for id := range model.Motors {
		motorIDs = append(motorIDs, id)
	}
	sort.Strings(motorIDs)

	// 9) Header timeseries
	header := []string{
		"t_seconds",
		"step",
		"recipe_step", // <-- fase atual do Controller (útil para análise)
		"tank_id",
		"volume_l",
		"level_pct",
		"temp_c",
		"pressure_bar",
	}

	for _, id := range valveIDs {
		header = append(header, id+"_open")
		header = append(header, id+"_flow")
	}
	for _, id := range motorIDs {
		header = append(header, id+"_state")
		header = append(header, id+"_active")
		header = append(header, id+"_rpm")
	}

	if err := w.Write(header); err != nil {
		log.Fatalf("write header: %v", err)
	}

	// Header events
	if err := ew.Write([]string{"t_seconds", "source", "event", "value"}); err != nil {
		log.Fatalf("write events header: %v", err)
	}

	// 10) Loop simulação
	totalSteps := int(model.DurationSeconds / model.DTSeconds)
	if totalSteps < 1 {
		totalSteps = 1
	}

	for i := 0; i < totalSteps; i++ {
		tNow := float64(i) * model.DTSeconds

		// 10.1) APLICA LÓGICA DE COMANDOS (Controller)
		//
		// O controller olha o estado atual (volume, pressão, etc.)
		// e decide quais comandos aplicar (abrir/fechar válvulas, ligar motores).
		ctrlRes, err := controller.Apply(model, tNow)
		if err != nil {
			log.Fatalf("controller apply: %v", err)
		}

		// (opcional) registrar eventos do controller no events.csv
		for _, ce := range ctrlRes.Events {
			_ = ew.Write([]string{
				fmt.Sprintf("%.3f", tNow),
				"controller",
				ce,
				"",
			})
		}

		// 10.2) Avança 1 step de simulação (Engine)
		res, err := engine.Step()
		if err != nil {
			log.Fatalf("engine step: %v", err)
		}

		// 10.3) Events dos sensores (engine)
		for _, ev := range res.Events {
			_ = ew.Write([]string{
				fmt.Sprintf("%.3f", ev.TimeSec),
				"sensor",
				fmt.Sprintf("%s:%s", ev.Tag, ev.Kind),
				fmt.Sprintf("%.6f", ev.Value),
			})
		}

		// 10.4) Timeseries (1 linha por tanque)
		for _, tankID := range tankIDs {
			tk, ok := res.Tanks[tankID]
			if !ok {
				continue
			}

			row := []string{
				fmt.Sprintf("%.3f", res.TimeSeconds),
				fmt.Sprintf("%d", res.Step),
				ctrlRes.StepName,
				tankID,
				fmt.Sprintf("%.6f", tk.VolumeL),
				fmt.Sprintf("%.6f", tk.LevelPct),
				fmt.Sprintf("%.6f", tk.TempC),
				fmt.Sprintf("%.6f", tk.PressureBar),
			}

			for _, vid := range valveIDs {
				vs, ok := res.Valves[vid]
				if !ok {
					row = append(row, "0", "0")
					continue
				}
				row = append(row, bool01(vs.Open), bool01(vs.Flow))
			}

			for _, mid := range motorIDs {
				ms, ok := res.Motors[mid]
				if !ok {
					row = append(row, "", "0", "0")
					continue
				}
				row = append(row, ms.State, bool01(ms.Active), fmt.Sprintf("%.3f", ms.RPM))
			}

			if err := w.Write(row); err != nil {
				log.Fatalf("write row: %v", err)
			}
		}

		// flush incremental
		w.Flush()
		if err := w.Error(); err != nil {
			log.Fatalf("timeseries flush: %v", err)
		}
		ew.Flush()
		if err := ew.Error(); err != nil {
			log.Fatalf("events flush: %v", err)
		}
	}

	log.Printf("OK: gerado %s", tsPath)
	log.Printf("OK: gerado %s", evPath)
}