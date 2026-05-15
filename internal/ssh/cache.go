package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	sshconfig "github.com/kevinburke/ssh_config"
	cache "github.com/patrickmn/go-cache"
	gossh "golang.org/x/crypto/ssh"
)

var (
	dialTCP           = net.DialTimeout
	authMethodBuilder = authMethodsForResolvedOptions
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

type resolvedConnectionOptions struct {
	Host          string
	Port          string
	Address       string
	Username      string
	Password      string
	PrivateKey    string
	IdentityFiles []string
}

func (opts resolvedConnectionOptions) CacheKey() string {
	if opts.Password != "" {
		return fmt.Sprintf("%s:%s:password:%s", opts.Address, opts.Username, opts.Password)
	}
	if opts.PrivateKey != "" {
		return fmt.Sprintf("%s:%s:private_key:%s", opts.Address, opts.Username, opts.PrivateKey)
	}
	if len(opts.IdentityFiles) > 0 {
		return fmt.Sprintf("%s:%s:identity_files:%s", opts.Address, opts.Username, strings.Join(opts.IdentityFiles, ","))
	}
	return fmt.Sprintf("%s:%s:auto_private_key", opts.Address, opts.Username)
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
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}

	resolvedOpts, err := resolveConnectionOptions(opts, homeDir)
	if err != nil {
		return nil, err
	}

	key := resolvedOpts.CacheKey()

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
	client, err := connectResolvedSSH(resolvedOpts, homeDir)
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

func (c *ConnectionCache) resolveCacheKey(opts Options) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户目录失败: %w", err)
	}

	resolvedOpts, err := resolveConnectionOptions(opts, homeDir)
	if err != nil {
		return "", err
	}

	return resolvedOpts.CacheKey(), nil
}

// ItemCount 返回缓存中的条目数
func (c *ConnectionCache) ItemCount() int {
	return c.cache.ItemCount()
}

// connectSSH 建立 SSH 连接
func connectSSH(opts Options) (*gossh.Client, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}

	resolvedOpts, err := resolveConnectionOptions(opts, homeDir)
	if err != nil {
		return nil, err
	}

	return connectResolvedSSH(resolvedOpts, homeDir)
}

func connectResolvedSSH(opts resolvedConnectionOptions, homeDir string) (*gossh.Client, error) {
	if err := probeTCPReachability(opts.Address, 30*time.Second); err != nil {
		return nil, err
	}

	config := &gossh.ClientConfig{
		User: opts.Username,
		Auth: []gossh.AuthMethod{},
		HostKeyCallback: func(hostname string, remote net.Addr, key gossh.PublicKey) error {
			return nil
		},
		Timeout: 30 * time.Second,
	}

	authMethods, err := authMethodBuilder(opts, homeDir)
	if err != nil {
		return nil, err
	}
	config.Auth = append(config.Auth, authMethods...)

	if len(config.Auth) == 0 {
		return nil, fmt.Errorf("未提供任何认证方式")
	}

	conn, err := gossh.Dial("tcp", opts.Address, config)
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

func authMethodsForResolvedOptions(opts resolvedConnectionOptions, homeDir string) ([]gossh.AuthMethod, error) {
	authMethods := make([]gossh.AuthMethod, 0, 2)
	parseErrors := make([]string, 0)

	if opts.PrivateKey != "" {
		authMethod, err := publicKeyAuthMethod(opts.PrivateKey)
		if err != nil {
			return nil, err
		}
		authMethods = append(authMethods, authMethod)
	}

	for _, keyPath := range opts.IdentityFiles {
		authMethod, keyErr := publicKeyAuthMethod(keyPath)
		if keyErr != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("%s: %v", filepath.Base(keyPath), keyErr))
			continue
		}
		authMethods = append(authMethods, authMethod)
	}

	if opts.Password != "" {
		authMethods = append(authMethods, gossh.Password(opts.Password))
	}

	if opts.PrivateKey == "" && opts.Password == "" && len(authMethods) == 0 {
		fallbackKeys, err := discoverPrivateKeys(filepath.Join(homeDir, ".ssh"))
		if err != nil {
			return nil, err
		}

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

func authMethodsForOptions(opts Options, homeDir string) ([]gossh.AuthMethod, error) {
	resolvedOpts, err := resolveConnectionOptions(opts, homeDir)
	if err != nil {
		return nil, err
	}

	return authMethodsForResolvedOptions(resolvedOpts, homeDir)
}

func resolveConnectionOptions(opts Options, homeDir string) (resolvedConnectionOptions, error) {
	resolved := resolvedConnectionOptions{
		Host:       opts.Host,
		Port:       "22",
		Username:   opts.Username,
		Password:   opts.Password,
		PrivateKey: opts.PrivateKey,
	}

	config, err := decodeSSHConfig(filepath.Join(homeDir, ".ssh", "config"))
	if err != nil {
		return resolvedConnectionOptions{}, err
	}

	if config != nil {
		if hostName, getErr := config.Get(opts.Host, "HostName"); getErr == nil && hostName != "" {
			resolved.Host = hostName
		}
		if resolved.Username == "" {
			if user, getErr := config.Get(opts.Host, "User"); getErr == nil && user != "" {
				resolved.Username = user
			}
		}
		if port, getErr := config.Get(opts.Host, "Port"); getErr == nil && port != "" {
			resolved.Port = port
		}
		if resolved.PrivateKey == "" {
			identityFiles, getErr := sshConfigValues(config, opts.Host, "IdentityFile")
			if getErr != nil {
				return resolvedConnectionOptions{}, fmt.Errorf("读取 SSH config 的 IdentityFile 失败: %w", getErr)
			}
			resolved.IdentityFiles = expandSSHPaths(identityFiles, homeDir)
		}
	}

	if resolved.Username == "" {
		resolved.Username = DefaultUsername()
	}
	resolved.Address = net.JoinHostPort(resolved.Host, resolved.Port)

	return resolved, nil
}

func decodeSSHConfig(configPath string) (*sshconfig.Config, error) {
	configFile, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("读取 SSH config 失败: %w", err)
	}
	defer configFile.Close()

	config, err := sshconfig.Decode(configFile)
	if err != nil {
		return nil, fmt.Errorf("解析 SSH config 失败: %w", err)
	}
	return config, nil
}

func sshConfigValues(config *sshconfig.Config, alias string, key string) ([]string, error) {
	method := reflect.ValueOf(config).MethodByName("GetAll")
	if method.IsValid() {
		results := method.Call([]reflect.Value{reflect.ValueOf(alias), reflect.ValueOf(key)})
		if len(results) == 2 && !results[1].IsNil() {
			if err, ok := results[1].Interface().(error); ok {
				return nil, err
			}
		}
		if len(results) > 0 {
			if values, ok := results[0].Interface().([]string); ok {
				return values, nil
			}
		}
	}

	value, err := config.Get(alias, key)
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}
	return []string{value}, nil
}

func expandSSHPaths(paths []string, homeDir string) []string {
	expanded := make([]string, 0, len(paths))
	for _, path := range paths {
		switch {
		case path == "~":
			expanded = append(expanded, homeDir)
		case strings.HasPrefix(path, "~/"):
			expanded = append(expanded, filepath.Join(homeDir, path[2:]))
		default:
			expanded = append(expanded, path)
		}
	}
	return expanded
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
