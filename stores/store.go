package stores

import (
	"fulcrum-test/config"
)

type Store interface {
	Save(event Event) error
	IsDuplicate(eventID string) bool
	Shutdown() error
}

func NewStores(cfg *config.Config) ([]Store, error) {
	var stores []Store

	if cfg.Stores.Postgres {
		ps, err := NewPostgresStore(cfg)
		if err != nil {
			return nil, err
		}
		stores = append(stores, ps)
	}

	if cfg.Stores.NDJSON {
		ns, err := NewNDJSONStore(cfg)
		if err != nil {
			return nil, err
		}
		stores = append(stores, ns)
	}

	return stores, nil
}
