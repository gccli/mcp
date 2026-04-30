package ssh

import (
	"testing"
	"time"
)

func TestNewConnectionCache(t *testing.T) {
	cache := NewConnectionCache()
	if cache == nil {
		t.Fatal("NewConnectionCache 返回 nil")
	}
}

func TestConnectionCacheKey(t *testing.T) {
	tests := []struct {
		name     string
		opts     Options
		expected string
	}{
		{
			name:     "密码认证",
			opts:     Options{Host: "192.168.1.1", Username: "root", Password: "test123"},
			expected: "192.168.1.1:root:password:test123",
		},
		{
			name:     "私钥认证",
			opts:     Options{Host: "192.168.1.1", Username: "root", PrivateKey: "/path/to/key"},
			expected: "192.168.1.1:root:private_key:/path/to/key",
		},
		{
			name:     "默认用户名",
			opts:     Options{Host: "192.168.1.1", Password: "test123"},
			expected: "192.168.1.1:root:password:test123",
		},
		{
			name:     "不同主机",
			opts:     Options{Host: "10.0.0.1", Username: "admin", Password: "pass"},
			expected: "10.0.0.1:admin:password:pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := cacheKey(tt.opts)
			if key != tt.expected {
				t.Errorf("cacheKey() = %q, want %q", key, tt.expected)
			}
		})
	}
}

func TestConnectionCacheGetAndSet(t *testing.T) {
	cache := NewConnectionCache()

	opts := Options{Host: "192.168.1.1", Username: "root", Password: "test123"}
	key := cacheKey(opts)

	// 缓存中不存在
	conn, ok := cache.Get(key)
	if ok {
		t.Error("期望缓存中不存在，但找到了")
	}
	if conn != nil {
		t.Error("期望 conn 为 nil")
	}

	// 创建模拟连接并存入缓存
	mockConn := &cachedConnection{
		client:  nil, // 实际测试中为 nil，因为我们不真正连接
		created: time.Now(),
	}
	cache.Set(key, mockConn)

	// 从缓存获取
	retrieved, ok := cache.Get(key)
	if !ok {
		t.Error("期望缓存中存在，但未找到")
	}
	if retrieved == nil {
		t.Fatal("期望 retrieved 不为 nil")
	}
	if retrieved.created.IsZero() {
		t.Error("期望 created 不为零值")
	}
}

func TestConnectionCacheExpiration(t *testing.T) {
	// 创建一个短 TTL 的缓存用于测试
	cache := NewConnectionCacheWithTTL(2*time.Second, 1*time.Second)

	opts := Options{Host: "192.168.1.1", Username: "root", Password: "test123"}
	key := cacheKey(opts)

	mockConn := &cachedConnection{
		client:  nil,
		created: time.Now(),
	}
	cache.Set(key, mockConn)

	// 立即获取，应该存在
	_, ok := cache.Get(key)
	if !ok {
		t.Error("刚存入缓存，期望存在")
	}

	// 等待过期
	time.Sleep(3 * time.Second)

	_, ok = cache.Get(key)
	if ok {
		t.Error("已过期，期望不存在")
	}
}

func TestConnectionCacheDifferentKeys(t *testing.T) {
	cache := NewConnectionCache()

	opts1 := Options{Host: "192.168.1.1", Username: "root", Password: "pass1"}
	opts2 := Options{Host: "192.168.1.2", Username: "root", Password: "pass2"}

	key1 := cacheKey(opts1)
	key2 := cacheKey(opts2)

	if key1 == key2 {
		t.Error("不同选项应该产生不同的缓存键")
	}

	mockConn1 := &cachedConnection{created: time.Now()}
	mockConn2 := &cachedConnection{created: time.Now()}

	cache.Set(key1, mockConn1)
	cache.Set(key2, mockConn2)

	conn1, ok1 := cache.Get(key1)
	conn2, ok2 := cache.Get(key2)

	if !ok1 || !ok2 {
		t.Error("两个缓存条目都应该存在")
	}
	if conn1 == conn2 {
		t.Error("两个缓存条目应该是不同的对象")
	}
}

func TestConnectionCacheCleanup(t *testing.T) {
	cache := NewConnectionCacheWithTTL(2*time.Second, 1*time.Second)

	opts := Options{Host: "192.168.1.1", Username: "root", Password: "test"}
	key := cacheKey(opts)

	mockConn := &cachedConnection{created: time.Now()}
	cache.Set(key, mockConn)

	// 等待清理周期
	time.Sleep(4 * time.Second)

	// 过期条目应被清理
	count := cache.ItemCount()
	if count != 0 {
		t.Errorf("期望缓存条目数为 0，但得到 %d", count)
	}
}
