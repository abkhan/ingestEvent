package stores

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"fulcrum-test/config"

	_ "github.com/lib/pq"
)

type PostgresStore struct {
	DB *sql.DB
}

func NewPostgresStore(cfg *config.Config) (*PostgresStore, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Database.Host, cfg.Database.Port, cfg.Database.User, cfg.Database.Password, cfg.Database.Name)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Create table if not exists
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS events (
		event_id TEXT PRIMARY KEY,
		tenant_id TEXT,
		user_id TEXT,
		session_id TEXT,
		event_type TEXT,
		properties JSONB,
		occurred_at TIMESTAMP
	)`)
	if err != nil {
		return nil, err
	}

	return &PostgresStore{DB: db}, nil
}

func (ps *PostgresStore) Save(event Event) error {
	propertiesJSON, err := json.Marshal(event.Properties)
	if err != nil {
		return err
	}
	_, err = ps.DB.Exec("INSERT INTO events (event_id, tenant_id, user_id, session_id, event_type, properties, occurred_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		event.EventID, event.TenantID, event.UserID, event.SessionID, event.EventType, propertiesJSON, event.OccurredAt)
	return err
}

func (ps *PostgresStore) IsDuplicate(eventID string) bool {
	var count int
	err := ps.DB.QueryRow("SELECT COUNT(*) FROM events WHERE event_id = $1 AND occurred_at > NOW() - INTERVAL '24 hours'", eventID).Scan(&count)
	return err == nil && count > 0
}

func (ps *PostgresStore) Shutdown() error {
	return ps.DB.Close()
}
