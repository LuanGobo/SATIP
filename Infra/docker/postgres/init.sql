-- Habilitar a extensão TimescaleDB
CREATE EXTENSION IF NOT EXISTS timescaledb;

-- Tabela de tipos de equipamentos
CREATE TABLE equipment_type (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL UNIQUE
);

INSERT INTO equipment_type (name) VALUES 
('tank'), 
('valve'), 
('motor'), 
('sensor');

-- Tabela de equipamentos
CREATE TABLE equipment (
    id TEXT PRIMARY KEY,
    tag TEXT NOT NULL,
    type_id INTEGER REFERENCES equipment_type(id),
    metadata JSONB
);

-- Tabela de usuários
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role TEXT DEFAULT 'operator'
);

-- Inserir um usuário inicial (senha: admin123 - apenas como exemplo)
INSERT INTO users (username, password_hash, role) 
VALUES ('admin', 'scrypt:32768:8:1$x$y', 'admin');

-- Tabela de histórico (Séries Temporais)
CREATE TABLE history (
    time TIMESTAMPTZ NOT NULL,
    equipment_id TEXT NOT NULL REFERENCES equipment(id),
    metric TEXT NOT NULL,
    val_f DOUBLE PRECISION,
    val_t TEXT
);

-- Transformar history em hypertable
SELECT create_hypertable('history', 'time', if_not_exists => TRUE);

-- Índices adicionais para performance
CREATE INDEX IF NOT EXISTS idx_history_equipment_metric ON history (equipment_id, metric, time DESC);
