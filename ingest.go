package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type Ingest struct {
	store   *Store
	metrics *Metrics
}

func NewIngest(store *Store, metrics *Metrics) *Ingest {
	return &Ingest{store: store, metrics: metrics}
}

func (i *Ingest) HandleSingle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		i.metrics.RequestDuration.WithLabelValues("/v1/events").Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		i.metrics.EventsTotal.WithLabelValues("", "invalid").Inc()
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if !i.validateEvent(event) {
		i.metrics.EventsTotal.WithLabelValues(event.TenantID, "invalid").Inc()
		http.Error(w, "Invalid event schema", http.StatusBadRequest)
		return
	}

	corrID := r.Header.Get("X-Request-ID")
	if corrID == "" {
		corrID = uuid.New().String()
	}
	log.Printf(`{"correlation_id": "%s", "event_id": "%s", "action": "received"}`, corrID, event.EventID)

	if i.store.IsDuplicate(event.EventID) {
		i.metrics.DroppedDuplicates.WithLabelValues(event.TenantID).Inc()
		i.metrics.EventsTotal.WithLabelValues(event.TenantID, "duplicate").Inc()
		log.Printf(`{"correlation_id": "%s", "event_id": "%s", "action": "dropped_duplicate"}`, corrID, event.EventID)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if err := i.store.SaveToDB(event); err != nil {
		log.Printf("DB error: %v", err)
		i.metrics.EventsTotal.WithLabelValues(event.TenantID, "error").Inc()
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	i.store.Enqueue(event)
	i.metrics.EventsTotal.WithLabelValues(event.TenantID, "success").Inc()
	log.Printf(`{"correlation_id": "%s", "event_id": "%s", "action": "enqueued"}`, corrID, event.EventID)
	w.WriteHeader(http.StatusAccepted)
}

func (i *Ingest) HandleBatch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		i.metrics.RequestDuration.WithLabelValues("/v1/events/batch").Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Events []Event `json:"events"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if len(req.Events) > 500 {
		http.Error(w, "Too many events", http.StatusBadRequest)
		return
	}

	results := make([]map[string]interface{}, len(req.Events))
	for idx, event := range req.Events {
		result := map[string]interface{}{"index": idx}
		if !i.validateEvent(event) {
			result["status"] = "invalid"
			i.metrics.EventsTotal.WithLabelValues("", "invalid").Inc()
		} else if i.store.IsDuplicate(event.EventID) {
			result["status"] = "duplicate"
			i.metrics.DroppedDuplicates.WithLabelValues(event.TenantID).Inc()
			i.metrics.EventsTotal.WithLabelValues(event.TenantID, "duplicate").Inc()
		} else {
			if err := i.store.SaveToDB(event); err != nil {
				result["status"] = "error"
				i.metrics.EventsTotal.WithLabelValues(event.TenantID, "error").Inc()
			} else {
				i.store.Enqueue(event)
				result["status"] = "success"
				i.metrics.EventsTotal.WithLabelValues(event.TenantID, "success").Inc()
			}
		}
		results[idx] = result
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
}

func (i *Ingest) validateEvent(event Event) bool {
	if event.EventID == "" || event.TenantID == "" || event.SessionID == "" || event.EventType == "" || event.OccurredAt == "" {
		return false
	}
	validTypes := map[string]bool{"page_view": true, "product_click": true, "add_to_cart": true, "purchase": true}
	if !validTypes[event.EventType] {
		return false
	}
	// Check properties size < 4KB
	props, _ := json.Marshal(event.Properties)
	if len(props) > 4096 {
		return false
	}
	return true
}
