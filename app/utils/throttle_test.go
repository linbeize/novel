package utils

import (
	"sync"
	"testing"
	"time"
)

// 同域名必须串行：并发调用 Wait 时不应出现两个请求同时在途
func TestThrottleSameHostSerialized(t *testing.T) {
	th := &Throttle{hosts: make(map[string]*hostState)}

	// 记录每个时刻在途请求数
	var mu sync.Mutex
	inflight := 0
	maxInflight := 0

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			th.Wait("https://example.com/a", 0, 0)

			mu.Lock()
			inflight++
			if inflight > maxInflight {
				maxInflight = inflight
			}
			mu.Unlock()

			time.Sleep(10 * time.Millisecond)

			mu.Lock()
			inflight--
			mu.Unlock()

			th.Done("https://example.com/a")
		}()
	}
	wg.Wait()

	if maxInflight != 1 {
		t.Fatalf("同域名应串行，实际最大并发 %d", maxInflight)
	}
}

// 不同域名之间不应互相阻塞
func TestThrottleDifferentHostsParallel(t *testing.T) {
	th := &Throttle{hosts: make(map[string]*hostState)}

	start := time.Now()
	var wg sync.WaitGroup

	for _, host := range []string{"https://a.com/x", "https://b.com/x", "https://c.com/x"} {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			th.Wait(u, 300*time.Millisecond, 0)
			time.Sleep(300 * time.Millisecond)
			th.Done(u)
		}(host)
	}
	wg.Wait()

	// 三个不同域名并行，总耗时应接近 600ms 而不是 1800ms
	if elapsed := time.Since(start); elapsed > 1200*time.Millisecond {
		t.Fatalf("不同域名应可并行，实际耗时 %v", elapsed)
	}
}

// 最小间隔生效：第二次请求必须等待 interval
func TestThrottleMinInterval(t *testing.T) {
	th := &Throttle{hosts: make(map[string]*hostState)}
	url := "https://example.com/a"

	th.Wait(url, 0, 0)
	th.Done(url)

	start := time.Now()
	th.Wait(url, 200*time.Millisecond, 0)
	th.Done(url)

	if elapsed := time.Since(start); elapsed < 190*time.Millisecond {
		t.Fatalf("最小间隔应生效，实际仅等待 %v", elapsed)
	}
}

// 冷却：被限流后，该域名应等待冷却结束
func TestThrottleCooldown(t *testing.T) {
	th := &Throttle{hosts: make(map[string]*hostState)}
	url := "https://example.com/a"

	th.Cooldown(url, 200*time.Millisecond)

	start := time.Now()
	th.Wait(url, 0, 0)
	th.Done(url)

	if elapsed := time.Since(start); elapsed < 190*time.Millisecond {
		t.Fatalf("冷却应生效，实际仅等待 %v", elapsed)
	}
}

// hostKey 解析
func TestHostKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"https://www.5566xs.com/0/106/", "www.5566xs.com"},
		{"http://Example.COM/x", "example.com"},
		{"https://a.com/1", "a.com"},
	}
	for _, c := range cases {
		if got := hostKey(c.in); got != c.want {
			t.Errorf("hostKey(%q)=%q, 期望 %q", c.in, got, c.want)
		}
	}
}
