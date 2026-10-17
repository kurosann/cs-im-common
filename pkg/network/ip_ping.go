package network

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-ping/ping"
)

type PingResult struct {
	Address   string
	Latency   float64
	Available bool
	Error     string
}

type PingOptions struct {
	Count      int
	Timeout    time.Duration
	Interval   time.Duration
	MaxRetries int
}

func PingBatch(ctx context.Context, addresses []string, options *PingOptions) []PingResult {
	if options == nil {
		options = &PingOptions{
			Count:    1,
			Timeout:  5 * time.Second,
			Interval: 0,
		}
	}

	resultsChan := make(chan PingResult, len(addresses))
	var wg sync.WaitGroup

	for _, addr := range addresses {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			result := pingSingle(ctx, address, options)
			resultsChan <- result
		}(addr)
	}

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var results []PingResult
	for result := range resultsChan {
		results = append(results, result)
	}

	sort.Slice(results, func(i, j int) bool {
		if !results[i].Available && results[j].Available {
			return false
		}
		if results[i].Available && !results[j].Available {
			return true
		}
		return results[i].Latency < results[j].Latency
	})

	return results
}

func pingSingle(ctx context.Context, address string, options *PingOptions) PingResult {
	var lastError string

	for attempt := 0; attempt <= options.MaxRetries; attempt++ {
		result := doPing(address, options)

		if result.Available {
			return result
		}

		lastError = result.Error

		if attempt < options.MaxRetries {
			time.Sleep(100 * time.Millisecond)
		}
	}

	return PingResult{
		Address:   address,
		Available: false,
		Error:     fmt.Sprintf("重试%d次后失败: %s", options.MaxRetries, lastError),
	}
}

func doPing(address string, options *PingOptions) PingResult {
	result := PingResult{
		Address: address,
	}

	pinger, err := ping.NewPinger(address)
	if err != nil {
		result.Available = false
		result.Error = fmt.Sprintf("创建ping失败: %v", err)
		return result
	}

	pinger.SetPrivileged(true)
	pinger.Count = options.Count
	pinger.Timeout = options.Timeout
	if options.Interval > 0 {
		pinger.Interval = options.Interval
	}

	err = pinger.Run()
	if err != nil {
		result.Available = false
		result.Error = fmt.Sprintf("ping执行失败: %v", err)
		return result
	}

	stats := pinger.Statistics()
	if stats.PacketsRecv > 0 {
		result.Available = true
		result.Latency = float64(stats.AvgRtt) / float64(time.Millisecond)
	} else {
		result.Available = false
		result.Error = "无响应包"
	}

	return result
}

func PingBatchSimple(addresses []string) []PingResult {
	ctx := context.Background()
	options := &PingOptions{
		Count:    1,
		Timeout:  5 * time.Second,
		Interval: 0,
	}
	return PingBatch(ctx, addresses, options)
}
