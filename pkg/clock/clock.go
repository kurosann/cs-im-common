package clock

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"
	_ "unsafe"

	"github.com/beevik/ntp"

	"vane-common/pkg/proxies"
	"vane-common/pkg/safe"
)

//go:noescape
//go:linkname nanotime runtime.nanotime
func nanotime() int64

var clock Clock = defClock{}

func SetClock(c Clock) {
	clock = c
}

// Clock 提供统一时间源。
//
// Now() 基于 runtime.nanotime()（CLOCK_MONOTONIC）推进，保证：
//   - 严格单调递增，不受 wall clock 跳变影响
//   - 系统时间前调/后调/自动校时均不影响返回值
//
// MTProto msg_id 使用注意：
//   - Now() 只作为时间基准
//   - 同一纳秒内多条消息的 msg_id 唯一性由上层自行递增保证
type Clock interface {
	Now() time.Time
	HasSynced() bool // NTP 至少完成过一次同步（defClock 恒为 true）
}

// ---- default ----

var _ Clock = (*defClock)(nil)

type defClock struct{}

func (defClock) Now() time.Time  { return time.Now() }
func (defClock) HasSynced() bool { return true } // 无 NTP 依赖，始终已同步

func Now() time.Time                  { return clock.Now() }
func Since(t time.Time) time.Duration { return Now().Sub(t) }
func HasSynced() bool                 { return clock.HasSynced() }

func GetClock() Clock { return clock }

// ---- ntpClock：无锁单调逻辑时钟 ----

var _ Clock = (*ntpClock)(nil)

// ntpAnchor 是一次 NTP 同步的纯数值快照：
//
//	ntpBase  = 该时刻的绝对标准 NTP 纳秒时间戳
//	syncMono = 该时刻对应的系统单调纳秒（CLOCK_MONOTONIC）
//	syncWall = 该时刻对应的系统挂钟纳秒（用于休眠检测）
//
// 时间推算（纯单调，不依赖挂钟）：
//
//	elapsed = nanotime() - syncMono
//	NTP时间  = ntpBase + elapsed
//
// 挂钟仅在 NewNtpClock 初始化时参与一次 ntpBase 的计算，
// 后续 sync() 中通过 monotonic 校验防线彻底免疫挂钟篡改。
// 全部使用 int64 运算，无 GC 分配。
type ntpAnchor struct {
	ntpBase  int64
	syncMono int64
	syncWall int64
}

// 三级校验阈值
const (
	smallDevThreshold = int64(500 * time.Millisecond) // ±500ms：正常抖动
	slewStep          = int64(100 * time.Millisecond) // 渐进追赶步长
	largeDevThreshold = int64(5 * time.Second)        // >5s：进入连续确认
)

const (
	sleepCheckInterval = int64(100 * time.Millisecond) // 休眠检测节流间隔
)

type ntpClock struct {
	addrs          []string
	idx            int // 仅 sync() 访问
	anchor         atomic.Pointer[ntpAnchor]
	ttl            time.Duration
	syncing        atomic.Int32 // 0 = idle, 1 = sync 进行中
	lastAttempt    atomic.Int64 // nanotime() of last sync launch (cooldown)
	lastSleepCheck atomic.Int64 // nanotime() of last sleep check (throttle)
	panicCount     atomic.Int32 // 极端偏差连续计数
	synced         atomic.Bool  // 至少完成过一次 NTP sync
}

func NewNtpClock(addrs []string, ttl time.Duration) Clock {
	c := &ntpClock{
		addrs: addrs,
		ttl:   ttl,
	}

	// 仅在初始化时信任一次系统挂钟，建立 ntpBase 起点。
	wall := time.Now().UnixNano()
	mono := nanotime()

	if len(addrs) == 0 {
		c.synced.Store(true) // 无 NTP 地址，直接标记已同步
		c.anchor.Store(&ntpAnchor{ntpBase: wall, syncMono: mono, syncWall: wall})
		return c
	}
	offset, idx, err := queryAny(addrs, -1)
	c.idx = idx
	if err == nil {
		c.synced.Store(true) // 初始化时已成功完成一次 NTP 同步
	}
	c.anchor.Store(&ntpAnchor{ntpBase: wall + int64(offset), syncMono: mono, syncWall: wall})
	return c
}

func (n *ntpClock) HasSynced() bool { return n.synced.Load() }

// Now 返回当前单调逻辑时间。
// 严格基于 monotonic elapsed 推进，不依赖 wall clock。
//
// 休眠检测每 100ms 节流一次（避免高频场景下 time.Now() 开销），
// 检测到系统休眠唤醒时回退到 wall clock 并立即触发 NTP 重同步。
func (n *ntpClock) Now() time.Time {
	a := n.anchor.Load()
	if a == nil {
		return time.Now()
	}

	nowMono := nanotime()
	elapsed := time.Duration(nowMono - a.syncMono)

	// 休眠检测节流：每 100ms 最多一次 time.Now() 调用
	if nowMono-n.lastSleepCheck.Load() > sleepCheckInterval {
		n.lastSleepCheck.Store(nowMono)
		if nowWall := time.Now().UnixNano(); nowWall-(a.syncWall+int64(elapsed)) > int64(2*time.Second) ||
			nowWall-(a.syncWall+int64(elapsed)) < -int64(2*time.Second) {
			if n.syncing.CompareAndSwap(0, 1) {
				n.synced.Store(false)
				n.lastAttempt.Store(nowMono)
				safe.Go(n.sync)
			}
			return time.Now()
		}
	}

	// 三重防重复：TTL + cooldown + CAS sync guard
	if elapsed > n.ttl &&
		time.Duration(nowMono-n.lastAttempt.Load()) > n.ttl &&
		n.syncing.CompareAndSwap(0, 1) {
		n.lastAttempt.Store(nowMono)
		safe.Go(n.sync)
	}

	ntpNano := a.ntpBase + int64(elapsed)
	return time.Unix(0, ntpNano)
}

// sync 异步同步 NTP。
//
// 三级校验防线——分级治理 + 连续确认：
//
//	T1 (±500ms)：正常抖动 → 直接信任 NTP，清零 panicCount
//	T2 (+500ms ~ +5s)：正向漂移 → Slew 渐进追赶 100ms，防止 msg_id 阶跃
//	T3 (负向或 >+5s)：极端偏差 → 连续确认机制
//	  - panicCount < 3：判定挂钟篡改，坚守 expectedNtp
//	  - panicCount >= 3：连续稳定，判定初始脏基准或长期漂移，吞下偏差强制收敛
func (n *ntpClock) sync() {
	defer n.syncing.Store(0)

	if len(n.addrs) == 0 {
		return
	}

	// 1. 查询 NTP
	newOffset, err := queryOffset(n.addrs[n.idx])
	if err != nil {
		if o, i, e := queryAny(n.addrs, n.idx); e == nil {
			newOffset, n.idx, err = o, i, nil
		}
	}
	if err != nil {
		return
	}

	// 2. 加载旧锚点
	oldAnchor := n.anchor.Load()
	if oldAnchor == nil {
		return
	}

	// 3. 捕获当前挂钟和单调时间
	nowMono := nanotime()
	nowWall := time.Now().UnixNano()

	// calculatedNtp：挂钟 + NTP offset 推算的"标准时间"
	calculatedNtp := nowWall + int64(newOffset)

	// expectedNtp：旧锚点 + 纯单调步进推算的"单调预期时间"
	expectedNtp := oldAnchor.ntpBase + (nowMono - oldAnchor.syncMono)

	// 4. 休眠检测：挂钟步进与单调步进偏差超过 2s 判定系统休眠唤醒
	wallDelta := nowWall - oldAnchor.syncWall
	monoDelta := nowMono - oldAnchor.syncMono
	if abs(wallDelta-monoDelta) > int64(2*time.Second) {
		// 系统休眠唤醒：单调时钟不可靠，直接信任 NTP 推算时间
		n.anchor.Store(&ntpAnchor{
			ntpBase:  calculatedNtp,
			syncMono: nowMono,
			syncWall: nowWall,
		})
		n.panicCount.Store(0)
		n.synced.Store(true)
		return
	}

	// 5. 三级校验防线
	diff := calculatedNtp - expectedNtp
	var ntpBase int64

	switch {
	case diff >= -smallDevThreshold && diff <= smallDevThreshold:
		// T1：微小偏差（±500ms 内）——正常网络抖动或常规漂移
		ntpBase = calculatedNtp
		n.panicCount.Store(0)

	case diff > smallDevThreshold && diff <= largeDevThreshold:
		// T2：中等正向偏差（500ms ~ 5s）——时钟稍微变快
		// Slew 渐进追赶，每次最多前拨 100ms，避免 msg_id 阶跃冲击
		ntpBase = expectedNtp + slewStep
		n.panicCount.Store(0)

	default:
		// T3：极端偏差（负向或超过 5s 正向）——连续确认机制
		count := n.panicCount.Add(1)
		if count < 3 {
			// 判定挂钟被篡改（前拨 1 年再拨回等），坚守单调时间线
			ntpBase = expectedNtp
		} else {
			// 连续 3 个同步周期均稳定出现极端偏差
			// 判定初始脏基准或长期漂移，吞下偏差强制收敛
			ntpBase = calculatedNtp
			n.panicCount.Store(0)
		}
	}

	// 6. 原子替换，零 GC 分配
	n.anchor.Store(&ntpAnchor{
		ntpBase:  ntpBase,
		syncMono: nowMono,
		syncWall: nowWall,
	})
	n.synced.Store(true)
}

// ---- NTP 查询 ----

func queryOffset(addr string) (time.Duration, error) {
	query, err := ntp.Query(addr)
	if err != nil {
		return 0, err
	}
	return query.ClockOffset, nil
}

func abs(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func queryAny(addrs []string, skipIdx int) (time.Duration, int, error) {
	for i, addr := range addrs {
		if i == skipIdx {
			continue
		}
		ips, err := proxies.GetGlobalDNSResolver().LookupIP(context.Background(), addr)
		if err == nil && len(ips) != 0 {
			addr = ips[0].String()
		}
		offset, err := queryOffset(addr)
		if err == nil {
			return offset, i, nil
		}
	}
	return 0, 0, fmt.Errorf("all ntp servers failed")
}
