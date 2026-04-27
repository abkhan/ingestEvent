package main

import (
	"os"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestIsDuplicate(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)
	metrics.Register(reg)
	store := &Store{db: db, metrics: metrics}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM events WHERE event_id = \\$1 AND occurred_at > NOW\\(\\) - INTERVAL '24 hours'").
		WithArgs("123").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	assert.True(t, store.IsDuplicate("123"))
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestFlushBatch(t *testing.T) {
	// Create temp file
	file, err := os.CreateTemp("", "test.ndjson")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)
	metrics.Register(reg)
	store := &Store{
		metrics: metrics,
		file:    file,
		batch:   []Event{{EventID: "123"}},
	}

	store.flushBatch()

	// Check file has content
	content, err := os.ReadFile(file.Name())
	assert.NoError(t, err)
	assert.Contains(t, string(content), "123")
	assert.Empty(t, store.batch)
}
