package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestValidateEvent(t *testing.T) {
	i := &Ingest{}
	event := Event{
		EventID:    "123",
		TenantID:   "tenant1",
		UserID:     "user1",
		SessionID:  "sess1",
		EventType:  "page_view",
		Properties: map[string]interface{}{"key": "value"},
		OccurredAt: "2023-01-01T00:00:00Z",
	}
	assert.True(t, i.validateEvent(event))

	event.EventType = "invalid"
	assert.False(t, i.validateEvent(event))
}

func TestHandleSingle(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)
	metrics.Register(reg)
	store := NewStore(db, metrics)
	ingest := NewIngest(store, metrics)

	// Mock duplicate check
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM events WHERE event_id = \\$1 AND occurred_at > NOW\\(\\) - INTERVAL '24 hours'").
		WithArgs("123").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Mock insert
	mock.ExpectExec("INSERT INTO events").
		WithArgs("123", "tenant1", "user1", "sess1", "page_view", sqlmock.AnyArg(), "2023-01-01T00:00:00Z").
		WillReturnResult(sqlmock.NewResult(1, 1))

	event := Event{
		EventID:    "123",
		TenantID:   "tenant1",
		UserID:     "user1",
		SessionID:  "sess1",
		EventType:  "page_view",
		Properties: map[string]interface{}{"key": "value"},
		OccurredAt: "2023-01-01T00:00:00Z",
	}
	body, _ := json.Marshal(event)
	req := httptest.NewRequest(http.MethodPost, "/v1/events", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ingest.HandleSingle(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestHandleBatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	assert.NoError(t, err)
	defer db.Close()

	reg := prometheus.NewRegistry()
	metrics := NewMetrics(reg)
	metrics.Register(reg)
	store := NewStore(db, metrics)
	ingest := NewIngest(store, metrics)

	events := []Event{
		{
			EventID:    "123",
			TenantID:   "tenant1",
			UserID:     "user1",
			SessionID:  "sess1",
			EventType:  "page_view",
			Properties: map[string]interface{}{"key": "value"},
			OccurredAt: "2023-01-01T00:00:00Z",
		},
	}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM events WHERE event_id = \\$1 AND occurred_at > NOW\\(\\) - INTERVAL '24 hours'").
		WithArgs("123").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectExec("INSERT INTO events").
		WithArgs("123", "tenant1", "user1", "sess1", "page_view", sqlmock.AnyArg(), "2023-01-01T00:00:00Z").
		WillReturnResult(sqlmock.NewResult(1, 1))

	reqBody := map[string]interface{}{"events": events}
	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/v1/events/batch", bytes.NewReader(body))
	w := httptest.NewRecorder()

	ingest.HandleBatch(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	results := resp["results"].([]interface{})
	assert.Equal(t, "success", results[0].(map[string]interface{})["status"])
	assert.NoError(t, mock.ExpectationsWereMet())
}
