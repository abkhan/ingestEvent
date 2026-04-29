package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	"fulcrum-test/stores"

	"github.com/stretchr/testify/assert"
)

func TestIntegrationBatch(t *testing.T) {
	fulcrumURL := os.Getenv("FULCRUM_URL")
	if fulcrumURL == "" {
		fulcrumURL = "http://localhost:8080"
	}

	events := []stores.Event{
		{
			EventID:    "int123",
			TenantID:   "tenant1",
			UserID:     "user1",
			SessionID:  "sess1",
			EventType:  "page_view",
			Properties: map[string]interface{}{"key": "value"},
			OccurredAt: "2023-01-01T00:00:00Z",
		},
	}

	reqBody := map[string]interface{}{"events": events}
	body, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", fulcrumURL+"/v1/events/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer default-key")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var respBody map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&respBody)
	results := respBody["results"].([]interface{})
	assert.Equal(t, "success", results[0].(map[string]interface{})["status"])
}
