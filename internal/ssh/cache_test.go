package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"encoding/pem"

	gossh "golang.org/x/crypto/ssh"
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
			name:     "自动私钥回退",
			opts:     Options{Host: "192.168.1.1"},
			expected: "192.168.1.1:root:auto_private_key",
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

func TestDiscoverPrivateKeys(t *testing.T) {
	tempDir := t.TempDir()
	sshDir := filepath.Join(tempDir, ".ssh")
	if err := os.Mkdir(sshDir, 0o700); err != nil {
		t.Fatalf("创建 .ssh 目录失败: %v", err)
	}

	keyA := filepath.Join(sshDir, "id_rsa")
	keyB := filepath.Join(sshDir, "id_ed25519")
	ignoredMode := filepath.Join(sshDir, "ignored_key")
	ignoredDir := filepath.Join(sshDir, "nested")

	writeValidPrivateKey(t, keyA, 0o400)
	writeValidPrivateKey(t, keyB, 0o400)
	writeValidPrivateKey(t, ignoredMode, 0o600)
	if err := os.Mkdir(ignoredDir, 0o400); err != nil {
		t.Fatalf("创建嵌套目录失败: %v", err)
	}

	keys, err := discoverPrivateKeys(sshDir)
	if err != nil {
		t.Fatalf("discoverPrivateKeys() 返回错误: %v", err)
	}

	sort.Strings(keys)
	expected := []string{keyB, keyA}
	if len(keys) != len(expected) {
		t.Fatalf("discoverPrivateKeys() 返回 %d 个结果, want %d", len(keys), len(expected))
	}
	for i := range expected {
		if keys[i] != expected[i] {
			t.Fatalf("discoverPrivateKeys()[%d] = %q, want %q", i, keys[i], expected[i])
		}
	}
}

func TestAuthMethodsForOptionsFallsBackToDiscoveredKeys(t *testing.T) {
	tempDir := t.TempDir()
	sshDir := filepath.Join(tempDir, ".ssh")
	if err := os.Mkdir(sshDir, 0o700); err != nil {
		t.Fatalf("创建 .ssh 目录失败: %v", err)
	}

	writeValidPrivateKey(t, filepath.Join(sshDir, "id_rsa"), 0o400)
	writeValidPrivateKey(t, filepath.Join(sshDir, "id_ed25519"), 0o400)
	writeValidPrivateKey(t, filepath.Join(sshDir, "ignored_key"), 0o600)

	authMethods, err := authMethodsForOptions(Options{Host: "192.168.1.1"}, tempDir)
	if err != nil {
		t.Fatalf("authMethodsForOptions() 返回错误: %v", err)
	}
	if len(authMethods) != 2 {
		t.Fatalf("authMethodsForOptions() 返回 %d 个认证方式, want 2", len(authMethods))
	}
}

func TestConnectSSHChecksTCPReachabilityBeforeAuth(t *testing.T) {
	originalDialTCP := dialTCP
	originalAuthMethodBuilder := authMethodBuilder
	defer func() {
		dialTCP = originalDialTCP
		authMethodBuilder = originalAuthMethodBuilder
	}()

	authMethodCalled := 0
	dialTCP = func(network, address string, timeout time.Duration) (net.Conn, error) {
		return nil, errors.New("network unreachable")
	}
	authMethodBuilder = func(opts Options, homeDir string) ([]gossh.AuthMethod, error) {
		authMethodCalled++
		return nil, nil
	}

	_, err := connectSSH(Options{Host: "192.168.1.1"})
	if err == nil {
		t.Fatal("期望 connectSSH 返回错误，但得到 nil")
	}
	if !strings.Contains(err.Error(), "TCP 连通性探测失败") {
		t.Fatalf("connectSSH() error = %q, want contain %q", err.Error(), "TCP 连通性探测失败")
	}
	if authMethodCalled != 0 {
		t.Fatalf("期望 TCP 探测失败后不进入认证构建，但调用了 %d 次", authMethodCalled)
	}
}

func writeValidPrivateKey(t *testing.T, path string, mode os.FileMode) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("生成测试私钥失败: %v", err)
	}

	encoded := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err := os.WriteFile(path, encoded, mode); err != nil {
		t.Fatalf("写入测试私钥失败: %v", err)
	}
	if err := os.Chmod(path, mode); err != nil {
		t.Fatalf("设置测试私钥权限失败: %v", err)
	}
}
