package stores

import (
	"encoding/json"
	"log"
	"os"
	"sync"
	"time"

	"fulcrum-test/config"
)

type NDJSONStore struct {
	file    *os.File
	batch   []Event
	timer   *time.Timer
	mu      sync.Mutex
	running bool
	wg      sync.WaitGroup
}

func NewNDJSONStore(cfg *config.Config) (*NDJSONStore, error) {
	os.MkdirAll("/data", 0755)
	file, err := os.OpenFile("/data/analytical.ndjson", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	ns := &NDJSONStore{
		file:    file,
		batch:   make([]Event, 0, 100),
		running: true,
	}
	ns.wg.Add(1)
	go ns.process()
	return ns, nil
}

func (ns *NDJSONStore) Save(event Event) error {
	ns.mu.Lock()
	defer ns.mu.Unlock()
	ns.batch = append(ns.batch, event)
	if len(ns.batch) >= 100 {
		ns.flushBatch()
		if ns.timer != nil {
			ns.timer.Reset(5 * time.Second)
		}
	}
	return nil
}

func (ns *NDJSONStore) IsDuplicate(eventID string) bool {
	// NDJSON doesn't check duplicates; that's handled by Postgres
	return false
}

func (ns *NDJSONStore) Shutdown() error {
	ns.mu.Lock()
	ns.running = false
	if ns.timer != nil {
		ns.timer.Stop()
	}
	ns.flushBatch()
	ns.mu.Unlock()
	ns.wg.Wait()
	return ns.file.Close()
}

func (ns *NDJSONStore) process() {
	defer ns.wg.Done()
	ns.timer = time.NewTimer(5 * time.Second)
	for {
		select {
		case <-ns.timer.C:
			ns.mu.Lock()
			if !ns.running {
				ns.mu.Unlock()
				return
			}
			if len(ns.batch) > 0 {
				ns.flushBatch()
			}
			ns.timer.Reset(5 * time.Second)
			ns.mu.Unlock()
		}
	}
}

func (ns *NDJSONStore) flushBatch() {
	if len(ns.batch) == 0 {
		return
	}
	data, _ := json.Marshal(ns.batch)
	data = append(data, '\n')
	_, err := ns.file.Write(data)
	if err != nil {
		log.Printf("Failed to write to analytical file: %v", err)
	}
	ns.file.Sync()
	ns.batch = ns.batch[:0]
}