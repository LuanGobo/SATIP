// Apps/device_app/cmd/simulator/main.go
package main

import (
	"context"
	"flag"
	"log"
	"math/rand"
	"os"
	"time"

	"device_app/internal/app"
	"device_app/internal/infra/db"
)

// jitterConnections aplica variação aleatória de corrida a corrida às vazões
// e temperaturas de entrada, alterando o tempo necessário para encher,
// esvaziar, aquecer e resfriar o tanque.
func jitterConnections(spec *app.PlantSpec, rng *rand.Rand) {
	for i := range spec.Connections {
		c := &spec.Connections[i]
		switch c.Type {
		case "product_inlet", "cip_inlet":
			c.Source.FlowLpm *= 1.0 + (rng.Float64()*2-1)*0.20
			c.Source.TempC += (rng.Float64()*2 - 1) * 3.0

		case "clean_air_inlet":
			c.Source.FlowNlpm *= 1.0 + (rng.Float64()*2-1)*0.20

		case "transfer_outlet":
			c.Source.NominalFlowLpm *= 1.0 + (rng.Float64()*2-1)*0.20

		case "steam_jacket":
			c.Source.TempC += (rng.Float64()*2 - 1) * 5.0
		}
	}
}

func main() {
	// CLI
	cfgPath := flag.String("config", "cmd/simulator/plant.yaml", "path to plant YAML")
	fouling := flag.Float64("fouling", 1.0, "fator de incrustacao da camisa (1.0 = limpa)")
	offsetH := flag.Float64("offset-h", 0, "deslocamento do instante inicial, em horas")
	seedFlag := flag.Int64("seed", 0, "semente aleatoria para variabilidade da corrida (0 = usa o relogio, diferente a cada execucao)")
	flag.Parse()

	seed := *seedFlag
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))
	log.Printf("Semente desta simulação: %d", seed)

	ctx := context.Background()

	// 1) Conexão com Banco de Dados
	connStr := os.Getenv("POSTGRES_URL")
	if connStr == "" {
		// fallback para desenvolvimento local se não estiver no docker
		connStr = "postgres://tcc_user:tcc_pass@localhost:5432/tcc?sslmode=disable"
	}

	database, err := db.NewDatabase(ctx, connStr)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer database.Close()

	// 2) Carrega spec
	spec, err := app.LoadPlantSpec(*cfgPath)
	if err != nil {
		log.Fatalf("load spec: %v", err)
	}

	// 2.1) Variabilidade natural entre corridas: vazões e temperaturas de
	// entrada variam, alterando os tempos de enchimento, esvaziamento,
	// aquecimento e resfriamento.
	jitterConnections(spec, rng)

	// 3) Build model
	model, err := app.BuildModel(spec)
	if err != nil {
		log.Fatalf("build model: %v", err)
	}

	for _, tk := range model.Tanks {
		tk.SetFoulingFactor(*fouling)
		// Variabilidade natural de corrida a corrida no tempo de aquecimento e
		// de resfriamento (±20% e ±25%), independente da falha de incrustação
		// controlada pela flag -fouling.
		tk.SetFoulingFactor(1.0 + (rng.Float64()*2-1)*0.20)
		tk.SetCoolingFactor(1.0 + (rng.Float64()*2-1)*0.25)
	}

	// 3.1) Falha aleatória de processo: com 35% de chance, o atuador da
	// válvula de vapor apresenta obstrução mecânica e não fecha por completo.
	// O controlador continua comandando o fechamento na banda de controle,
	// mas a passagem residual de vapor faz a camisa subir além do setpoint e
	// cruzar os limites de alarme de TIT-02 — condição que o controle sozinho
	// não consegue corrigir e que só é diagnosticável pelo histórico.
	if steamValve, ok := model.Valves["xv-06"]; ok && rng.Float64() < 0.35 {
		stuck := 0.30 + rng.Float64()*0.25 // 30% a 55% de abertura residual
		steamValve.SetStuckMinimum(stuck)
		log.Printf("Falha simulada: atuador de XV-06 obstruido, abertura residual de %.0f%%", stuck*100)
	}

	// Variação do tempo de curso dos atuadores entre corridas
	for _, v := range model.Valves {
		v.SetStrokeSeconds(8.0 + rng.Float64()*10.0) // 8 a 18 s
	}

	// 4) Garantir equipamentos no banco
	for _, tk := range spec.Equipment.Tanks {
		err := database.EnsureEquipment(ctx, tk.ID, tk.Tag, "tank", map[string]interface{}{"capacity": tk.CapacityL})
		if err != nil {
			log.Fatalf("ensure tank %s: %v", tk.ID, err)
		}
	}
	for _, v := range spec.Equipment.Valves {
		err := database.EnsureEquipment(ctx, v.ID, v.Tag, "valve", map[string]interface{}{"service": v.Service})
		if err != nil {
			log.Fatalf("ensure valve %s: %v", v.ID, err)
		}
	}
	for _, m := range spec.Equipment.Motors {
		err := database.EnsureEquipment(ctx, m.ID, m.Tag, "motor", map[string]interface{}{"has_reverse": m.HasReverse})
		if err != nil {
			log.Fatalf("ensure motor %s: %v", m.ID, err)
		}
	}
	for _, s := range spec.Equipment.Sensors {
		err := database.EnsureEquipment(ctx, s.ID, s.Tag, "sensor", map[string]interface{}{"kind": s.Kind, "deadband": s.Deadband})
		if err != nil {
			log.Fatalf("ensure sensor %s: %v", s.ID, err)
		}
	}
	if err := database.EnsureEquipment(ctx, "controller", "CTRL-01", "controller", nil); err != nil {
		log.Fatalf("ensure controller: %v", err)
	}

	// 5) Engine
	engine, err := app.NewEngine(model)
	if err != nil {
		log.Fatalf("new engine: %v", err)
	}
	engine.SetRNG(rng)

	// 6) Controller
	controller := app.NewController("tk-01")

	// 7) Loop simulação
	totalSteps := int(model.DurationSeconds / model.DTSeconds)
	if totalSteps < 1 {
		totalSteps = 1
	}

	// Tempo base para o banco (simulando tempo real ou histórico)
	startTime := time.Now().Truncate(time.Second).
		Add(time.Duration(*offsetH * float64(time.Hour)))

	log.Printf("Iniciando simulação (%d steps)...", totalSteps)

	batchInterval := 100 // flush a cada 100 ticks

	for i := 0; i < totalSteps; i++ {
		tNowSim := float64(i) * model.DTSeconds
		tDB := startTime.Add(time.Duration(tNowSim) * time.Second)

		// 7.1) Controller
		ctrlRes, err := controller.Apply(model, tNowSim)
		if err != nil {
			log.Fatalf("controller apply: %v", err)
		}

		// Registrar eventos do controller (on-change por natureza)
		for _, ce := range ctrlRes.Events {
			_ = database.LogOnChange(ctx, tDB, "controller", "event", ce)
		}
		_ = database.LogOnChange(ctx, tDB, "controller", "recipe_step", ctrlRes.StepName)

		// 7.2) Engine Step
		res, err := engine.Step()
		if err != nil {
			log.Fatalf("engine step: %v", err)
		}

		// 7.3) Sensores Alarmes (on-change por natureza)
		for _, ev := range res.Events {
			sensorID := ev.Tag
			for _, s := range spec.Equipment.Sensors {
				if s.Tag == ev.Tag {
					sensorID = s.ID
					break
				}
			}
			_ = database.LogOnChange(ctx, tDB, sensorID, ev.Kind, ev.Value)
		}

		// 7.4) Estado dos Tanques — batch a cada tick
		for id, tk := range res.Tanks {
			database.AddToBatch(tDB, id, "volume_l", tk.VolumeL)
			database.AddToBatch(tDB, id, "level_pct", tk.LevelPct)
			database.AddToBatch(tDB, id, "temp_c", tk.TempC)
			database.AddToBatch(tDB, id, "pressure_bar", tk.PressureBar)
		}

		// 7.4a) Realimenta o controlador com o alarme HH de TIT-02 apurado
		// neste tick, para reagir no próximo (defasagem de 1 tick, como um
		// ciclo de varredura real).
		productH := false
		if s, ok := res.Sensors["tit-01"]; ok {
			productH = s.EnabledH && s.AlarmH
		}
		jacketH := false
		if s, ok := res.Sensors["tit-02"]; ok {
			jacketH = s.EnabledH && s.AlarmH
		}
		controller.SetAlarmFeedback(productH, jacketH)

		// 7.4b) Sensor readings + alarm states — batch a cada tick
		for id, ss := range res.Sensors {
			database.AddToBatch(tDB, id, "value", ss.Value)
			// Alarmes: só loga quando ativo (valor = leitura do sensor, sobrepõe a linha)
			if ss.EnabledLL && ss.AlarmLL {
				database.AddToBatch(tDB, id, "alarm_ll", ss.Value)
			}
			if ss.EnabledL && ss.AlarmL {
				database.AddToBatch(tDB, id, "alarm_l", ss.Value)
			}
			if ss.EnabledH && ss.AlarmH {
				database.AddToBatch(tDB, id, "alarm_h", ss.Value)
			}
			if ss.EnabledHH && ss.AlarmHH {
				database.AddToBatch(tDB, id, "alarm_hh", ss.Value)
			}
		}

		// 7.5) Válvulas — batch a cada tick
		for id, vs := range res.Valves {
			openVal := 0.0
			if vs.Open {
				openVal = 1.0
			}
			database.AddToBatch(tDB, id, "is_open", openVal)
			database.AddToBatch(tDB, id, "position", vs.Position)
		}

		// 7.6) Motores — batch a cada tick
		for id, ms := range res.Motors {
			activeVal := 0.0
			if ms.Active {
				activeVal = 1.0
			}
			database.AddToBatch(tDB, id, "active", activeVal)
			database.AddToBatch(tDB, id, "rpm", ms.RPM)
		}

		// Flush batch periodicamente
		if (i+1)%batchInterval == 0 || i == totalSteps-1 {
			if err := database.FlushBatch(ctx); err != nil {
				log.Fatalf("flush batch at step %d: %v", i, err)
			}
			if (i+1)%(batchInterval*50) == 0 {
				log.Printf("Progresso: %d/%d steps (%.0f%%)", i+1, totalSteps, float64(i+1)/float64(totalSteps)*100)
			}
		}
	}

	// Flush final
	if err := database.FlushBatch(ctx); err != nil {
		log.Fatalf("flush final batch: %v", err)
	}

	log.Printf("Simulação concluída com sucesso. Dados salvos no PostgreSQL.")
}