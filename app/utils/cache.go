// Copyright 2017 Vckai Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"sync"
	"time"
)

// 进程内 TTL 缓存
//
// 背景：本站首页单次请求要执行 10 余次数据库查询，其中「排行榜」与
// 「最新更新」需对 nov_novel 全表排序。在没有缓存的情况下，每次访问
// 都会重复执行同样的查询。此处提供一个进程内缓存，用于缓存这类
// 「读多写少、允许短暂不一致」的榜单数据。
//
// 取舍说明：
//   - 纯内存实现，不引入 Redis 等外部依赖，部署方式不变；
//   - 多实例部署时各进程缓存独立，属于可接受的偏差；
//   - 采用固定 TTL 而非主动失效，因此数据最多延迟一个 TTL 周期，
//     对榜单类展示数据（分钟级）完全够用。
type cacheItem struct {
	val      interface{}
	expireAt time.Time
}

type TTLCache struct {
	mu    sync.RWMutex
	items map[string]cacheItem
}

var (
	defaultCache *TTLCache
	cacheOnce    sync.Once
)

// 获取默认缓存实例（单例）
func Cache() *TTLCache {
	cacheOnce.Do(func() {
		defaultCache = NewTTLCache()
	})

	return defaultCache
}

func NewTTLCache() *TTLCache {
	return &TTLCache{
		items: make(map[string]cacheItem),
	}
}

// 读取缓存，未命中或已过期返回 nil, false
func (this *TTLCache) Get(key string) (interface{}, bool) {
	this.mu.RLock()
	it, ok := this.items[key]
	this.mu.RUnlock()

	if !ok {
		return nil, false
	}

	if time.Now().After(it.expireAt) {
		// 过期则删除，避免内存堆积
		this.mu.Lock()
		delete(this.items, key)
		this.mu.Unlock()

		return nil, false
	}

	return it.val, true
}

// 写入缓存
func (this *TTLCache) Set(key string, val interface{}, ttl time.Duration) {
	this.mu.Lock()
	this.items[key] = cacheItem{
		val:      val,
		expireAt: time.Now().Add(ttl),
	}
	this.mu.Unlock()
}

// 删除指定键
func (this *TTLCache) Delete(key string) {
	this.mu.Lock()
	delete(this.items, key)
	this.mu.Unlock()
}

// 按前缀删除（键通常携带参数，如 novel:newups:12）
func (this *TTLCache) DeletePrefix(prefix string) {
	this.mu.Lock()
	for k := range this.items {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			delete(this.items, k)
		}
	}
	this.mu.Unlock()
}

// 清空全部缓存
func (this *TTLCache) Flush() {
	this.mu.Lock()
	this.items = make(map[string]cacheItem)
	this.mu.Unlock()
}

// 清理过期项，可周期性调用
func (this *TTLCache) GC() {
	now := time.Now()

	this.mu.Lock()
	for k, v := range this.items {
		if now.After(v.expireAt) {
			delete(this.items, k)
		}
	}
	this.mu.Unlock()
}

// 便捷方法：命中则返回缓存，否则执行 fn 并缓存结果
// 注意：fn 在锁外执行，避免慢查询阻塞其它键的读取
func (this *TTLCache) Remember(key string, ttl time.Duration, fn func() interface{}) interface{} {
	if v, ok := this.Get(key); ok {
		return v
	}

	v := fn()

	this.Set(key, v, ttl)

	return v
}
