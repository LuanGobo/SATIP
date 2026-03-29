package db

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// HistoryRow represents a single row to batch-insert into history.
type HistoryRow struct {
	Time        time.Time
	EquipmentID string
	Metric      string
	ValF        float64
}

type Database struct {
	pool      *pgxpool.Pool
	lastVals  map[string]interface{}
	lastMutex sync.Mutex
	batch     []HistoryRow
}

func NewDatabase(ctx context.Context, connStr string) (*Database, error) {
	config, err := pgxpool.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse config: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("connect to db: %v", err)
	}

	return &Database{
		pool:     pool,
		lastVals: make(map[string]interface{}),
	}, nil
}

func (db *Database) Close() {
	db.pool.Close()
}

// EnsureEquipment garante que o equipamento e seu tipo existam no banco.
func (db *Database) EnsureEquipment(ctx context.Context, id, tag, typeName string, metadata map[string]interface{}) error {
	var typeID int
	err := db.pool.QueryRow(ctx, "SELECT id FROM equipment_type WHERE name = $1", typeName).Scan(&typeID)
	if err != nil {
		return fmt.Errorf("get equipment type %s: %v", typeName, err)
	}

	metaJSON, _ := json.Marshal(metadata)

	_, err = db.pool.Exec(ctx, `
		INSERT INTO equipment (id, tag, type_id, metadata)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET tag = $2, metadata = $4
	`, id, tag, typeID, metaJSON)

	return err
}

// LogOnChange registra o valor apenas se houver mudança.
func (db *Database) LogOnChange(ctx context.Context, t time.Time, equipmentID, metric string, val interface{}) error {
	db.lastMutex.Lock()
	key := fmt.Sprintf("%s:%s", equipmentID, metric)
	last, exists := db.lastVals[key]
	
	changed := !exists
	if exists {
		changed = (last != val)
	}

	if !changed {
		db.lastMutex.Unlock()
		return nil
	}

	db.lastVals[key] = val
	db.lastMutex.Unlock()

	// Inserir no histórico
	var err error
	switch v := val.(type) {
	case float64:
		_, err = db.pool.Exec(ctx, "INSERT INTO history (time, equipment_id, metric, val_f) VALUES ($1, $2, $3, $4)", t, equipmentID, metric, v)
	case string:
		_, err = db.pool.Exec(ctx, "INSERT INTO history (time, equipment_id, metric, val_t) VALUES ($1, $2, $3, $4)", t, equipmentID, metric, v)
	case bool:
		fv := 0.0
		if v {
			fv = 1.0
		}
		_, err = db.pool.Exec(ctx, "INSERT INTO history (time, equipment_id, metric, val_f) VALUES ($1, $2, $3, $4)", t, equipmentID, metric, fv)
	default:
		_, err = db.pool.Exec(ctx, "INSERT INTO history (time, equipment_id, metric, val_t) VALUES ($1, $2, $3, $4)", t, equipmentID, fmt.Sprintf("%v", v))
	}

	return err
}

// LogAlways registra o valor incondicionalmente (sem verificação de mudança).
func (db *Database) LogAlways(ctx context.Context, t time.Time, equipmentID, metric string, val float64) error {
	_, err := db.pool.Exec(ctx,
		"INSERT INTO history (time, equipment_id, metric, val_f) VALUES ($1, $2, $3, $4)",
		t, equipmentID, metric, val)
	return err
}

// AddToBatch acumula uma linha no buffer interno para inserção em lote.
func (db *Database) AddToBatch(t time.Time, equipmentID, metric string, val float64) {
	db.batch = append(db.batch, HistoryRow{Time: t, EquipmentID: equipmentID, Metric: metric, ValF: val})
}

// FlushBatch insere todas as linhas acumuladas usando COPY (muito mais rápido que INSERTs individuais).
func (db *Database) FlushBatch(ctx context.Context) error {
	if len(db.batch) == 0 {
		return nil
	}

	rows := make([][]interface{}, len(db.batch))
	for i, r := range db.batch {
		rows[i] = []interface{}{r.Time, r.EquipmentID, r.Metric, r.ValF}
	}

	_, err := db.pool.CopyFrom(
		ctx,
		pgx.Identifier{"history"},
		[]string{"time", "equipment_id", "metric", "val_f"},
		pgx.CopyFromRows(rows),
	)

	db.batch = db.batch[:0] // limpa o buffer
	return err
}

// LogSensorOnChange lida especificamente com deadband para sensores.
func (db *Database) LogSensorOnChange(ctx context.Context, t time.Time, equipmentID, metric string, val float64, deadband float64) error {
	db.lastMutex.Lock()
	key := fmt.Sprintf("%s:%s", equipmentID, metric)
	last, exists := db.lastVals[key]
	
	changed := !exists
	if exists {
		lastF := last.(float64)
		diff := lastF - val
		if diff < 0 {
			diff = -diff
		}
		changed = diff >= deadband
	}

	if !changed {
		db.lastMutex.Unlock()
		return nil
	}

	db.lastVals[key] = val
	db.lastMutex.Unlock()

	_, err := db.pool.Exec(ctx, "INSERT INTO history (time, equipment_id, metric, val_f) VALUES ($1, $2, $3, $4)", t, equipmentID, metric, val)
	return err
}
