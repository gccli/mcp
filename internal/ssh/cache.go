package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	cache "github.com/patrickmn/go-cache"
	gossh "golang.org/x/crypto/ssh"
)

var (
	dialTCP           = net.DialTimeout
	authMethodBuilder = authMethodsForOptions
)

const (
	// 默认缓存 TTL：90 秒
	defaultCacheTTL = 90 * time.Second
	// 默认清理间隔：60 秒
	defaultCleanupInterval = 60 * time.Second
)

// cachedConnection 缓存的 SSH 连接
type cachedConnection struct {
	client  *gossh.Client
	created time.Time
}

// ConnectionCache SSH 连接缓存
type ConnectionCache struct {
	cache *cache.Cache
	ttl   time.Duration
}

// NewConnectionCache 创建默认 TTL (90秒) 的连接缓存
func NewConnectionCache() *ConnectionCache {
	return NewConnectionCacheWithTTL(defaultCacheTTL, defaultCleanupInterval)
}

// NewConnectionCacheWithTTL 创建指定 TTL 的连接缓存
func NewConnectionCacheWithTTL(ttl, cleanupInterval time.Duration) *ConnectionCache {
	return &ConnectionCache{
		cache: cache.New(ttl, cleanupInterval),
		ttl:   ttl,
	}
}

// cacheKey 生成缓存键
func cacheKey(opts Options) string {
	username := opts.Username
	if username == "" {
		username = DefaultUsername()
	}
	if opts.Password != "" {
		return fmt.Sprintf("%s:%s:password:%s", opts.Host, username, opts.Password)
	}
	if opts.PrivateKey == "" {
		return fmt.Sprintf("%s:%s:auto_private_key", opts.Host, username)
	}
	return fmt.Sprintf("%s:%s:private_key:%s", opts.Host, username, opts.PrivateKey)
}

// Get 从缓存获取连接
func (c *ConnectionCache) Get(key string) (*cachedConnection, bool) {
	item, found := c.cache.Get(key)
	if !found {
		return nil, false
	}
	conn, ok := item.(*cachedConnection)
	if !ok {
		return nil, false
	}
	return conn, true
}

// Set 将连接存入缓存
func (c *ConnectionCache) Set(key string, conn *cachedConnection) {
	c.cache.Set(key, conn, c.ttl)
}

// GetOrCreate 获取缓存的连接，如果不存在则创建新连接
func (c *ConnectionCache) GetOrCreate(opts Options) (*gossh.Client, error) {
	key := cacheKey(opts)

	// 尝试从缓存获取
	cached, found := c.Get(key)
	if found && cached.client != nil {
		// 验证连接是否仍然有效
		session, err := cached.client.NewSession()
		if err == nil {
			session.Close()
			return cached.client, nil
		}
		// 连接已失效，从缓存中删除
		c.cache.Delete(key)
	}

	// 创建新连接
	client, err := connectSSH(opts)
	if err != nil {
		return nil, err
	}

	// 存入缓存
	c.Set(key, &cachedConnection{
		client:  client,
		created: time.Now(),
	})

	return client, nil
}

// ItemCount 返回缓存中的条目数
func (c *ConnectionCache) ItemCount() int {
	return c.cache.ItemCount()
}

// connectSSH 建立 SSH 连接
func connectSSH(opts Options) (*gossh.Client, error) {
	username := opts.Username
	if username == "" {
		username = DefaultUsername()
	}

	addr := fmt.Sprintf("%s:22", opts.Host)
	if err := probeTCPReachability(addr, 30*time.Second); err != nil {
		return nil, err
	}

	config := &gossh.ClientConfig{
		User: username,
		Auth: []gossh.AuthMethod{},
		HostKeyCallback: func(hostname string, remote net.Addr, key gossh.PublicKey) error {
			return nil
		},
		Timeout: 30 * time.Second,
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}

	authMethods, err := authMethodBuilder(opts, homeDir)
	if err != nil {
		return nil, err
	}
	config.Auth = append(config.Auth, authMethods...)

	if len(config.Auth) == 0 {
		return nil, fmt.Errorf("未提供任何认证方式")
	}

	conn, err := gossh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("连接 SSH 服务器失败: %w", err)
	}

	return conn, nil
}

func probeTCPReachability(addr string, timeout time.Duration) error {
	conn, err := dialTCP("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("TCP 连通性探测失败: %w", err)
	}

	if closeErr := conn.Close(); closeErr != nil {
		return fmt.Errorf("关闭 TCP 探测连接失败: %w", closeErr)
	}

	return nil
}

func authMethodsForOptions(opts Options, homeDir string) ([]gossh.AuthMethod, error) {
	authMethods := make([]gossh.AuthMethod, 0, 2)

	if opts.PrivateKey != "" {
		authMethod, err := publicKeyAuthMethod(opts.PrivateKey)
		if err != nil {
			return nil, err
		}
		authMethods = append(authMethods, authMethod)
	}

	if opts.Password != "" {
		authMethods = append(authMethods, gossh.Password(opts.Password))
	}

	if opts.PrivateKey == "" && opts.Password == "" {
		fallbackKeys, err := discoverPrivateKeys(filepath.Join(homeDir, ".ssh"))
		if err != nil {
			return nil, err
		}

		parseErrors := make([]string, 0)
		for _, keyPath := range fallbackKeys {
			authMethod, keyErr := publicKeyAuthMethod(keyPath)
			if keyErr != nil {
				parseErrors = append(parseErrors, fmt.Sprintf("%s: %v", filepath.Base(keyPath), keyErr))
				continue
			}
			authMethods = append(authMethods, authMethod)
		}

		if len(authMethods) == 0 {
			if len(parseErrors) > 0 {
				return nil, fmt.Errorf("未找到可用的自动私钥认证方式: %s", strings.Join(parseErrors, "; "))
			}
			return nil, fmt.Errorf("未提供任何认证方式，且 ~/.ssh 下不存在权限为 0400 的私钥文件")
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("未提供任何认证方式")
	}

	return authMethods, nil
}

func publicKeyAuthMethod(privateKeyPath string) (gossh.AuthMethod, error) {
	key, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %w", err)
	}

	signer, err := gossh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("解析私钥失败: %w", err)
	}

	return gossh.PublicKeys(signer), nil
}

func discoverPrivateKeys(sshDir string) ([]string, error) {
	entries, err := os.ReadDir(sshDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 ~/.ssh 目录失败: %w", err)
	}

	privateKeys := make([]string, 0)
	for _, entry := range entries {
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		if !info.Mode().IsRegular() || info.Mode().Perm() != 0o400 {
			continue
		}
		privateKeys = append(privateKeys, filepath.Join(sshDir, entry.Name()))
	}

	sort.Strings(privateKeys)
	return privateKeys, nil
}
