package publish

import (
	"context"
	"sync"
	"vane-common/pkg/idutil"

	"vane-common/pkg/logger"

	"go.uber.org/zap"
)

type ISubscriberStore interface {
	RecordSubscribe(ctx context.Context, topic string) error
	RemoveSubscribe(ctx context.Context, topic string) error
	QueryAllSubcribe(ctx context.Context) (endpoints map[string][]string, err error)
}

type subscriberManager struct {
	subscriberStore *subscriberStore
	subscriber      map[string]map[string]chan []byte // e.g. topic -> id -> chan
	subscriberLock  sync.RWMutex
}

func newSubscriberManager() *subscriberManager {
	return &subscriberManager{
		subscriberStore: newSubscriberStore(),
		subscriber:      make(map[string]map[string]chan []byte),
	}
}

func (s *subscriberManager) Subscribe(ctx context.Context, topic string, cache ...int) (id string, ch chan []byte, err error) {
	s.subscriberLock.Lock()
	defer s.subscriberLock.Unlock()
	cacheSize := 100
	if len(cache) > 0 {
		cacheSize = cache[0]
	}
	ch = make(chan []byte, cacheSize)
	if _, ok := s.subscriber[topic]; !ok {
		s.subscriber[topic] = make(map[string]chan []byte)
	}
	id = idutil.GenShortId()
	s.subscriber[topic][id] = ch

	if err := s.subscriberStore.RecordSubscribe(ctx, topic); err != nil {
		delete(s.subscriber, topic)
		return "", nil, err
	}
	logger.Debug(ctx, "subscribe", zap.String("topic", topic), zap.String("id", id))

	return id, ch, nil
}
func (s *subscriberManager) RemoveSubscribe(ctx context.Context, topic string, id string) error {
	s.subscriberLock.Lock()
	defer s.subscriberLock.Unlock()
	if _, ok := s.subscriber[topic]; !ok {
		return nil
	}
	ch, ok := s.subscriber[topic][id]
	if !ok {
		return nil
	}
	if len(s.subscriber[topic]) == 0 {
		if err := s.subscriberStore.RemoveSubscribe(ctx, topic); err != nil {
			return err
		}
	}
	close(ch)
	delete(s.subscriber[topic], id)
	return nil
}

func (s *subscriberManager) QuerySubcribe(ctx context.Context, topic string) (endpointsMap map[string]bool, err error) {
	subcribeMap, err := s.subscriberStore.QueryAllSubcribe(ctx)
	if err != nil {
		return nil, err
	}
	endpointsMap = make(map[string]bool)
	for _, endpoints := range subcribeMap[topic] {
		endpointsMap[endpoints] = true
	}
	return endpointsMap, nil
}
func (s *subscriberManager) UpdateSubcribe(ctx context.Context, topic string, endpoints []string) error {
	return s.subscriberStore.UpdateSubcribe(ctx, topic, endpoints)
}

func (s *subscriberManager) SendLocalSubcribe(ctx context.Context, topic string, msg []byte) {
	s.subscriberLock.RLock()
	defer s.subscriberLock.RUnlock()
	chMap, ok := s.subscriber[topic]
	if !ok {
		s.subscriberStore.RemoveSubscribe(ctx, topic)
		return
	}
	for _, ch := range chMap {
		ch <- msg
	}
}

func (s *subscriberManager) SyncSubcribe(ctx context.Context) error {
	return s.subscriberStore.SyncSubcribe(ctx)
}
