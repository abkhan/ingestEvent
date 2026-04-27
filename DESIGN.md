# Events Ingress Path Design Document

## 1. Real Dual-Store Integration

In production, replace PostgreSQL with Bigtable for operational reads and the NDJSON file with BigQuery for analytics.

**Data Path:**
- Events ingested via Ingestion Service, 
    Rest/HTTP, with validation -- and then
        -- Written to Bigtable with event_id as row key for fast lookups.
        -- streamed to Pub/Sub, then to Dataflow for transformation, and loaded into BigQuery via streaming inserts.
- Backfill: 
    -- BigQuery to Bigtable: Use BigQuery's time travel 
    -- Bigtable to Bigquery: export data to Google Cloud Storage, then load into BigQuery.

- Schema Evolution: 
    -- BigQuery: add migration job when a field has to be added
    -- Bigtable: Use Bigtable's column families for flexible properties.

**Delivery Guarantees:**
- At-least-once: Use Pub/Sub's ack mechanism and idempotent writes.
- Exactly-once: Implement deduplication, without stopping writes in Dataflow using event_id.

## 2. Operating on Call

**SLOs:**
- Latency: 95% of requests <100ms 
    -> (alert on >250ms for 5min).
- Error Rate: <1% 5xx responses 
    -> (alert on >3% for 10min).

## 3. Multi-Tenancy Under Load

**Bottlenecks:**
- Shared database connections pool exhaustion.
- Single analytical file contention.

**Isolation Mechanisms:**
- Per-tenant rate limits (e.g., 1000 req/s via Redis).
- Backpressure: Queue depth per tenant, drop excess.
- Cost Attribution: Metrics per tenant, bill based on usage.