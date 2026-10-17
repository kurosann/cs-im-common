package depend

import (
	"time"

	"github.com/avast/retry-go"
)

// Retry 执行函数，失败时重试
// attempts: 最大尝试次数
// delay: 每次重试的间隔
func Retry(attempts int, delay time.Duration, fn func() error) error {
	return retry.Do(
		fn,
		retry.Attempts(uint(attempts)),
		retry.Delay(delay),
		retry.DelayType(retry.FixedDelay),
		retry.LastErrorOnly(true),
	)
}
