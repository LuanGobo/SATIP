package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
)

var dbPool *pgxpool.Pool

func main() {
	ctx := context.Background()
	connStr := os.Getenv("POSTGRES_URL")
	if connStr == "" {
		connStr = "postgres://tcc_user:tcc_pass@timescaledb:5432/tcc?sslmode=disable"
	}

	var err error
	dbPool, err = pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer dbPool.Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type"},
	}))

	r.Get("/api/equipment", getEquipment)
	r.Get("/api/history/{id}", getHistory)
	r.Post("/api/simulation/start", startSimulation)
	r.Delete("/api/simulation/data", clearSimulationData)
	r.Get("/api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	log.Println("Dashboard API starting on :8080...")
	http.ListenAndServe(":8080", r)
}

func getEquipment(w http.ResponseWriter, r *http.Request) {
	rows, err := dbPool.Query(r.Context(), `
		SELECT e.id, e.tag, et.name as type, e.metadata
		FROM equipment e
		JOIN equipment_type et ON e.type_id = et.id
	`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	equipments := []map[string]interface{}{}
	for rows.Next() {
		var id, tag, eqType string
		var metadata []byte
		rows.Scan(&id, &tag, &eqType, &metadata)

		var meta map[string]interface{}
		json.Unmarshal(metadata, &meta)

		equipments = append(equipments, map[string]interface{}{
			"id":       id,
			"tag":      tag,
			"type":     eqType,
			"metadata": meta,
		})
	}

	json.NewEncoder(w).Encode(equipments)
}

func getHistory(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	metric := r.URL.Query().Get("metric")

	var query string
	var args []interface{}

	if metric != "" {
		query = `
			SELECT time, metric, val_f, val_t
			FROM history
			WHERE equipment_id = $1 AND metric = $2
			ORDER BY time ASC
		`
		args = []interface{}{id, metric}
	} else {
		query = `
			SELECT time, metric, val_f, val_t
			FROM history
			WHERE equipment_id = $1
			ORDER BY time ASC
		`
		args = []interface{}{id}
	}

	rows, err := dbPool.Query(r.Context(), query, args...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var t time.Time
		var m string
		var valF *float64
		var valT *string
		rows.Scan(&t, &m, &valF, &valT)

		val := interface{}(nil)
		if valF != nil {
			val = *valF
		} else if valT != nil {
			val = *valT
		}

		history = append(history, map[string]interface{}{
			"time":   t,
			"metric": m,
			"value":  val,
		})
	}

	json.NewEncoder(w).Encode(history)
}

func clearSimulationData(w http.ResponseWriter, r *http.Request) {
	_, err := dbPool.Exec(r.Context(), "DELETE FROM history")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Simulation data cleared.")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

func startSimulation(w http.ResponseWriter, r *http.Request) {
	// Clear previous data before starting
	_, err := dbPool.Exec(r.Context(), "DELETE FROM history")
	if err != nil {
		log.Printf("Failed to clear history: %v", err)
	}

	go func() {
		log.Println("Starting simulation execution...")
		cmd := exec.Command("/app/simulator")
		cmd.Env = os.Environ()
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("Simulation error: %v, Output: %s", err, string(output))
		} else {
			log.Println("Simulation completed successfully.")
		}
	}()

	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}
