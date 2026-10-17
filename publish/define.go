package publish

import (
	"context"
	"time"

	"github.com/avast/retry-go"
)

var (
	sm *subscriberManager
	pm *publishManager
	ss ISubscriberStore = &emptySubscriberStore{}
)

func setSubscriber(manager *subscriberManager) {
	sm = manager
}

func setPublisher(manager *publishManager) {
	pm = manager
}

func SetSubscriberStore(store ISubscriberStore) {
	ss = store
}

func defRetry(ctx context.Context, f func() error) error {
	return retry.Do(f, retry.Attempts(3), retry.Delay(100*time.Millisecond), retry.Context(ctx), retry.LastErrorOnly(true))
}

func Update(ctx context.Context, topic string, endpoints []string) error {
	return defRetry(ctx, func() error {
		return sm.UpdateSubcribe(ctx, topic, endpoints)
	})
}

// Publish 发布消息到所有订阅者
func Publish(ctx context.Context, topic string, data []byte) error {
	return defRetry(ctx, func() error {
		return pm.Send(ctx, topic, data)
	})
}

func PublishLocal(ctx context.Context, topic string, data []byte) {
	sm.SendLocalSubcribe(ctx, topic, data)
}

// Subscribe 订阅消息
func Subscribe(ctx context.Context, topic string) (ch chan []byte, cancel func() error, err error) {
	var id string
	if err = defRetry(ctx, func() error {
		id, ch, err = sm.Subscribe(ctx, topic)
		return err
	}); err != nil {
		return nil, nil, err
	}
	cancel = func() error {
		return sm.RemoveSubscribe(context.WithoutCancel(ctx), topic, id)
	}
	return ch, cancel, nil
}

func SyncSubcribe(ctx context.Context) error {
	return defRetry(ctx, func() error {
		if sm == nil {
			return nil
		}
		return sm.SyncSubcribe(ctx)
	})
}

// Unsubscribe 取消订阅消息
func Unsubscribe(ctx context.Context, topic string, id string) error {
	return defRetry(ctx, func() error {
		return sm.RemoveSubscribe(ctx, topic, id)
	})
}

type emptySubscriberStore struct{}

func (e *emptySubscriberStore) SyncSubcribe(ctx context.Context) error {
	return nil
}

func (e *emptySubscriberStore) RecordSubscribe(ctx context.Context, topic string) error {
	return nil
}

func (e *emptySubscriberStore) RemoveSubscribe(ctx context.Context, topic string) error {
	return nil
}

func (e *emptySubscriberStore) QueryAllSubcribe(ctx context.Context) (endpoints map[string][]string, err error) {
	return make(map[string][]string), nil
}
