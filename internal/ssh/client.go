package ssh

import (
	"bytes"
	"fmt"

	gossh "golang.org/x/crypto/ssh"
)

// Client SSH 客户端，使用缓存连接执行命令
type Client struct {
	opts  Options
	cache *ConnectionCache
}

// NewClient 创建新的 SSH 客户端（使用缓存连接）
func NewClient(opts Options, cache *ConnectionCache) *Client {
	username := opts.Username
	if username == "" {
		username = DefaultUsername()
	}
	opts.Username = username
	return &Client{
		opts:  opts,
		cache: cache,
	}
}

// ExecResult 命令执行结果
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// Exec 在远程主机上执行命令（使用缓存连接）
func (c *Client) Exec(command string) (*ExecResult, error) {
	client, err := c.cache.GetOrCreate(c.opts)
	if err != nil {
		return nil, err
	}

	session, err := client.NewSession()
	if err != nil {
		// session 创建失败，可能连接已失效，清除缓存并重试
		c.cache.cache.Delete(cacheKey(c.opts))
		client, err = c.cache.GetOrCreate(c.opts)
		if err != nil {
			return nil, err
		}
		session, err = client.NewSession()
		if err != nil {
			return nil, fmt.Errorf("创建 SSH session 失败: %w", err)
		}
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	err = session.Run(command)
	result := &ExecResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*gossh.ExitError); ok {
			result.ExitCode = exitErr.ExitStatus()
		} else {
			return nil, fmt.Errorf("执行命令失败: %w", err)
		}
	}

	return result, nil
}

// buildSudoCommand 构建 sudo 前缀命令
func (c *Client) buildSudoCommand(command string) string {
	return fmt.Sprintf("sudo %s", command)
}

// SudoExec 在远程主机上以 sudo 执行命令
func (c *Client) SudoExec(command string) (*ExecResult, error) {
	return c.Exec(c.buildSudoCommand(command))
}
