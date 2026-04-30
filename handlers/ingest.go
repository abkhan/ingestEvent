package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"fulcrum-test/stores"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
)

type IngestHandler struct {
	stores  []stores.Store
	metrics *Metrics
}

func NewIngestHandler(stores []stores.Store, metrics *Metrics) *IngestHandler {
	return &IngestHandler{stores: stores, metrics: metrics}
}

func (ih *IngestHandler) HandleSingle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		ih.metrics.RequestDuration.WithLabelValues("/v1/events").Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event stores.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		ih.metrics.EventsTotal.WithLabelValues("", "invalid").Inc()
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if !ih.ValidateEvent(event) {
		ih.metrics.EventsTotal.WithLabelValues(event.TenantID, "invalid").Inc()
		http.Error(w, "Invalid event schema", http.StatusBadRequest)
		return
	}

	corrID := r.Header.Get("X-Request-ID")
	if corrID == "" {
		corrID = uuid.New().String()
	}
	log.Printf(`{"correlation_id": "%s", "event_id": "%s", "action": "received"}`, corrID, event.EventID)

	// Check duplicate using the first store (assuming Postgres is first)
	isDup := false
	for _, store := range ih.stores {
		if store.IsDuplicate(event.EventID) {
			isDup = true
			break
		}
	}
	if isDup {
		ih.metrics.DroppedDuplicates.WithLabelValues(event.TenantID).Inc()
		ih.metrics.EventsTotal.WithLabelValues(event.TenantID, "duplicate").Inc()
		log.Printf(`{"correlation_id": "%s", "event_id": "%s", "action": "dropped_duplicate"}`, corrID, event.EventID)
		w.WriteHeader(http.StatusAccepted)
		return
	}

	// Save to all stores
	for _, store := range ih.stores {
		if err := store.Save(event); err != nil {
			log.Printf("Store error: %v", err)
			ih.metrics.EventsTotal.WithLabelValues(event.TenantID, "error").Inc()
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
	}

	ih.metrics.EventsTotal.WithLabelValues(event.TenantID, "success").Inc()
	log.Printf(`{"correlation_id": "%s", "event_id": "%s", "action": "enqueued"}`, corrID, event.EventID)
	w.WriteHeader(http.StatusAccepted)
}

func (ih *IngestHandler) HandleBatch(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		ih.metrics.RequestDuration.WithLabelValues("/v1/events/batch").Observe(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Events []stores.Event `json:"events"`
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
		if !ih.ValidateEvent(event) {
			result["status"] = "invalid"
			ih.metrics.EventsTotal.WithLabelValues("", "invalid").Inc()
		} else {
			isDup := false
			for _, store := range ih.stores {
				if store.IsDuplicate(event.EventID) {
					isDup = true
					break
				}
			}
			if isDup {
				result["status"] = "duplicate"
				ih.metrics.DroppedDuplicates.WithLabelValues(event.TenantID).Inc()
				ih.metrics.EventsTotal.WithLabelValues(event.TenantID, "duplicate").Inc()
			} else {
				err := false
				for _, store := range ih.stores {
					if saveErr := store.Save(event); saveErr != nil {
						err = true
						break
					}
				}
				if err {
					result["status"] = "error"
					ih.metrics.EventsTotal.WithLabelValues(event.TenantID, "error").Inc()
				} else {
					result["status"] = "success"
					ih.metrics.EventsTotal.WithLabelValues(event.TenantID, "success").Inc()
				}
			}
		}
		results[idx] = result
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
}

func (ih *IngestHandler) ValidateEvent(event stores.Event) bool {
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

// Metrics struct (moved from metrics.go)
type Metrics struct {
	EventsTotal              *prometheus.CounterVec
	RequestDuration          *prometheus.HistogramVec
	AnalyticalFlushBatchSize prometheus.Histogram
	AnalyticalFlushDuration  prometheus.Histogram
	DroppedDuplicates        *prometheus.CounterVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	useDefault := false
	if reg == nil {
		reg = prometheus.DefaultRegisterer
		useDefault = true
	}
	m := &Metrics{
		EventsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "ingest_events_total",
			Help: "Total number of ingested events",
		}, []string{"tenant", "status"}),
		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "ingest_request_duration_seconds",
			Help:    "Request duration in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"route"}),
		AnalyticalFlushBatchSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "analytical_flush_batch_size",
			Help:    "Batch size for analytical flushes",
			Buckets: prometheus.DefBuckets,
		}),
		AnalyticalFlushDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "analytical_flush_duration_seconds",
			Help:    "Duration of analytical flushes in seconds",
			Buckets: prometheus.DefBuckets,
		}),
		DroppedDuplicates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "dropped_duplicates_total",
			Help: "Total number of dropped duplicate events",
		}, []string{"tenant"}),
	}
	// Register all metrics
	collectors := []prometheus.Collector{
		m.EventsTotal,
		m.RequestDuration,
		m.AnalyticalFlushBatchSize,
		m.AnalyticalFlushDuration,
		m.DroppedDuplicates,
	}

	if useDefault {
		prometheus.MustRegister(collectors...)
	} else {
		for _, c := range collectors {
			if err := reg.Register(c); err != nil {
				log.Fatalf("failed to register metric: %v", err)
			}
		}
	}

	// Pre-create zero-value label combinations so vector metrics are visible on /metrics
	m.EventsTotal.WithLabelValues("unknown", "success")
	m.RequestDuration.WithLabelValues("/startup")
	m.DroppedDuplicates.WithLabelValues("unknown")

	log.Println("All metrics registered successfully")
	return m
}
