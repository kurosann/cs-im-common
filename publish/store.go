package publish

import (
	"context"
	"sync"
)

var _ ISubscriberStore = (*subscriberStore)(nil)

type subscriberStore struct {
	cache     map[string][]string
	cacheLock sync.RWMutex
}

func newSubscriberStore() *subscriberStore {
	return &subscriberStore{
		cache: make(map[string][]string),
	}
}

func (s *subscriberStore) QueryAllSubcribe(ctx context.Context) (endpoints map[string][]string, err error) {
	s.cacheLock.RLock()
	defer s.cacheLock.RUnlock()
	if len(s.cache) == 0 {
		if err = s.syncSubcribe(ctx); err != nil {
			return nil, err
		}
	}
	copyEndpoints := make(map[string][]string)
	for topic, endpoints := range s.cache {
		copyEndpoints[topic] = make([]string, len(endpoints))
		copy(copyEndpoints[topic], endpoints)
	}
	return copyEndpoints, nil
}

func (s *subscriberStore) RecordSubscribe(ctx context.Context, topic string) error {
	return ss.RecordSubscribe(ctx, topic)
}

func (s *subscriberStore) RemoveSubscribe(ctx context.Context, topic string) error {
	return ss.RemoveSubscribe(ctx, topic)
}

func (s *subscriberStore) UpdateSubcribe(ctx context.Context, topic string, endpoints []string) error {
	s.cacheLock.Lock()
	defer s.cacheLock.Unlock()
	s.cache[topic] = endpoints
	return nil
}

func (s *subscriberStore) SyncSubcribe(ctx context.Context) error {
	s.cacheLock.Lock()
	defer s.cacheLock.Unlock()

	return s.syncSubcribe(ctx)
}
func (s *subscriberStore) syncSubcribe(ctx context.Context) error {
	endpoints, err := ss.QueryAllSubcribe(ctx)
	if err != nil {
		return err
	}
	s.cache = endpoints
	return nil
}
