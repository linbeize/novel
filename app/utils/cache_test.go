package utils

import (
	"sync"
	"testing"
	"time"
)

// 缓存基础读写
func TestTTLCacheSetGet(t *testing.T) {
	c := NewTTLCache()

	c.Set("k", "v", time.Minute)

	v, ok := c.Get("k")
	if !ok || v.(string) != "v" {
		t.Fatalf("期望命中 k=v，实际 ok=%v v=%v", ok, v)
	}

	// 未设置的键
	if _, ok := c.Get("none"); ok {
		t.Fatal("未设置的键不应命中")
	}
}

// 过期后应失效
func TestTTLCacheExpire(t *testing.T) {
	c := NewTTLCache()

	c.Set("k", 1, 30*time.Millisecond)

	if _, ok := c.Get("k"); !ok {
		t.Fatal("过期前应命中")
	}

	time.Sleep(60 * time.Millisecond)

	if _, ok := c.Get("k"); ok {
		t.Fatal("过期后不应命中")
	}
}

// 按前缀删除（榜单缓存键带 size 参数）
func TestTTLCacheDeletePrefix(t *testing.T) {
	c := NewTTLCache()

	c.Set("novel:ranks:9", 1, time.Minute)
	c.Set("novel:ranks:20", 2, time.Minute)
	c.Set("novel:newups:12", 3, time.Minute)

	c.DeletePrefix("novel:ranks")

	if _, ok := c.Get("novel:ranks:9"); ok {
		t.Fatal("前缀删除后 novel:ranks:9 不应存在")
	}
	if _, ok := c.Get("novel:ranks:20"); ok {
		t.Fatal("前缀删除后 novel:ranks:20 不应存在")
	}
	// 不同前缀不受影响
	if _, ok := c.Get("novel:newups:12"); !ok {
		t.Fatal("不同前缀不应被删除")
	}
}

// Remember 只执行一次取数逻辑
func TestTTLCacheRemember(t *testing.T) {
	c := NewTTLCache()

	calls := 0
	for i := 0; i < 5; i++ {
		c.Remember("k", time.Minute, func() interface{} {
			calls++
			return "v"
		})
	}

	if calls != 1 {
		t.Fatalf("Remember 应只执行一次取数，实际 %d 次", calls)
	}
}

// 并发读写下不应出现数据竞争
func TestTTLCacheConcurrent(t *testing.T) {
	c := NewTTLCache()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.Set("k", n, time.Minute)
			c.Get("k")
			c.DeletePrefix("k")
		}(i)
	}
	wg.Wait()
}
