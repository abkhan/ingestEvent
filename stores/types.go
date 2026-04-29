package stores

type Event struct {
	EventID    string                 `json:"event_id"`
	TenantID   string                 `json:"tenant_id"`
	UserID     string                 `json:"user_id"`
	SessionID  string                 `json:"session_id"`
	EventType  string                 `json:"event_type"`
	Properties map[string]interface{} `json:"properties"`
	OccurredAt string                 `json:"occurred_at"`
}
