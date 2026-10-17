package network

import (
	"context"
	"testing"
	"time"
)

func TestPingBatch(t *testing.T) {
	addresses := []string{
		"8.8.8.8",
		"8.8.4.4",
		"1.1.1.1",
		"114.114.114.114",
	}

	ctx := context.Background()
	options := &PingOptions{
		Count:    10,
		Timeout:  5 * time.Second,
		Interval: 0,
	}

	results := PingBatch(ctx, addresses, options)

	if len(results) != len(addresses) {
		t.Errorf("期望返回 %d 个结果，实际返回 %d 个", len(addresses), len(results))
	}

	for _, result := range results {
		if result.Address == "" {
			t.Errorf("结果中地址为空")
		}
		if result.Available {
			if result.Latency < 0 {
				t.Errorf("地址 %s 的延迟为负数: %f", result.Address, result.Latency)
			}
		} else {
			if result.Error == "" {
				t.Errorf("地址 %s 不可用但错误信息为空", result.Address)
			}
		}
		t.Logf("地址: %s, 可用: %v, 延迟: %.2fms, 错误: %s", result.Address, result.Available, result.Latency, result.Error)
	}

	for i := 1; i < len(results); i++ {
		if results[i-1].Available && results[i].Available {
			if results[i-1].Latency > results[i].Latency {
				t.Errorf("结果未按延迟排序: %s(%.2fms) 在 %s(%.2fms) 之前",
					results[i-1].Address, results[i-1].Latency,
					results[i].Address, results[i].Latency)
			}
		}
	}
}

func TestPingBatchSimple(t *testing.T) {
	addresses := []string{
		"8.8.8.8",
		"8.8.4.4",
	}

	results := PingBatchSimple(addresses)

	if len(results) != len(addresses) {
		t.Errorf("期望返回 %d 个结果，实际返回 %d 个", len(addresses), len(results))
	}

	for _, result := range results {
		t.Logf("地址: %s, 可用: %v, 延迟: %.2fms", result.Address, result.Available, result.Latency)
	}
}

func TestPingBatchWithRetry(t *testing.T) {
	addresses := []string{
		"8.8.8.8",
		"192.0.2.1",
	}

	ctx := context.Background()
	options := &PingOptions{
		Count:      1,
		Timeout:    3 * time.Second,
		Interval:   0,
		MaxRetries: 2,
	}

	results := PingBatch(ctx, addresses, options)

	if len(results) != len(addresses) {
		t.Errorf("期望返回 %d 个结果，实际返回 %d 个", len(addresses), len(results))
	}

	for _, result := range results {
		t.Logf("地址: %s, 可用: %v, 延迟: %.2fms, 错误: %s", result.Address, result.Available, result.Latency, result.Error)
	}
}

func TestPingBatchWithCount(t *testing.T) {
	addresses := []string{
		"8.8.8.8",
	}

	ctx := context.Background()
	options := &PingOptions{
		Count:      3,
		Timeout:    5 * time.Second,
		Interval:   0,
		MaxRetries: 0,
	}

	results := PingBatch(ctx, addresses, options)

	if len(results) != 1 {
		t.Errorf("期望返回 1 个结果，实际返回 %d 个", len(results))
	}

	for _, result := range results {
		t.Logf("地址: %s, 可用: %v, 延迟: %.2fms", result.Address, result.Available, result.Latency)
	}
}

func TestPingBatchWithTimeout(t *testing.T) {
	addresses := []string{
		"192.0.2.1",
		"8.8.8.8",
	}

	ctx := context.Background()
	options := &PingOptions{
		Count:    1,
		Timeout:  2 * time.Second,
		Interval: 0,
	}

	results := PingBatch(ctx, addresses, options)

	if len(results) != len(addresses) {
		t.Errorf("期望返回 %d 个结果，实际返回 %d 个", len(addresses), len(results))
	}

	for _, result := range results {
		if result.Address == "192.0.2.1" {
			if result.Available {
				t.Errorf("地址 %s 应该不可用", result.Address)
			}
			if result.Error == "" {
				t.Errorf("地址 %s 应该有错误信息", result.Address)
			}
		}
		t.Logf("地址: %s, 可用: %v, 延迟: %.2fms, 错误: %s", result.Address, result.Available, result.Latency, result.Error)
	}
}
