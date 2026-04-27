package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"
)

type Event struct {
	EventID    string                 `json:"event_id"`
	TenantID   string                 `json:"tenant_id"`
	UserID     string                 `json:"user_id"`
	SessionID  string                 `json:"session_id"`
	EventType  string                 `json:"event_type"`
	Properties map[string]interface{} `json:"properties"`
	OccurredAt string                 `json:"occurred_at"`
}

type Store struct {
	db      *sql.DB
	metrics *Metrics
	events  chan Event
	wg      sync.WaitGroup
	file    *os.File
	batch   []Event
	timer   *time.Timer
	mu      sync.Mutex
}

func NewStore(db *sql.DB, metrics *Metrics) *Store {
	os.MkdirAll("./data", 0755)
	file, err := os.OpenFile("./data/analytical.ndjson", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	s := &Store{
		db:      db,
		metrics: metrics,
		events:  make(chan Event, 1000), // buffer
		file:    file,
		batch:   make([]Event, 0, 100),
	}
	s.wg.Add(1)
	go s.process()
	return s
}

func (s *Store) Enqueue(event Event) {
	s.events <- event
}

func (s *Store) process() {
	defer s.wg.Done()
	s.timer = time.NewTimer(5 * time.Second)
	for {
		select {
		case event, ok := <-s.events:
			if !ok {
				s.flushBatch()
				return // channel closed, exit -- expected during shutdown
			}
			s.mu.Lock()
			s.batch = append(s.batch, event)
			if len(s.batch) >= 100 {
				s.flushBatch()
				s.timer.Reset(5 * time.Second)
			}
			s.mu.Unlock()
		case <-s.timer.C:
			s.mu.Lock()
			if len(s.batch) > 0 {
				s.flushBatch()
			}
			s.timer.Reset(5 * time.Second)
			s.mu.Unlock()
		}
	}
}

func (s *Store) flushBatch() {
	if len(s.batch) == 0 {
		return
	}
	start := time.Now()
	// Write to file atomically
	data, _ := json.Marshal(s.batch)
	data = append(data, '\n')
	_, err := s.file.Write(data)
	if err != nil {
		log.Printf("Failed to write to analytical file: %v", err)
		return
	}
	s.file.Sync() // ensure written
	s.metrics.AnalyticalFlushBatchSize.Observe(float64(len(s.batch)))
	s.metrics.AnalyticalFlushDuration.Observe(time.Since(start).Seconds())
	s.batch = s.batch[:0] // reset
}

// IsDuplicate checks in db if an event with the same ID has been ingested in the last 24 hours
func (s *Store) IsDuplicate(eventID string) bool {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM events WHERE event_id = $1 AND occurred_at > NOW() - INTERVAL '24 hours'", eventID).Scan(&count)
	return err == nil && count > 0
}

// SaveToDB inserts the event into the PostgreSQL database - db funcs are here, but called from ingest.go to keep separation of concerns
func (s *Store) SaveToDB(event Event) error {
	propertiesJSON, err := json.Marshal(event.Properties)
	if err != nil {
		return err
	}
	_, err = s.db.Exec("INSERT INTO events (event_id, tenant_id, user_id, session_id, event_type, properties, occurred_at) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		event.EventID, event.TenantID, event.UserID, event.SessionID, event.EventType, propertiesJSON, event.OccurredAt)
	return err
}

func (s *Store) Shutdown(ctx context.Context) {
	close(s.events)
	s.wg.Wait()
	s.file.Close()
}
