package ssh

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Server SSH MCP Server（启动时不连接，调用工具时才连接，使用缓存）
type Server struct {
	cache *ConnectionCache
}

// NewServer 创建新的 SSH MCP Server（内置连接缓存，TTL 90秒）
func NewServer() (*Server, error) {
	return &Server{
		cache: NewConnectionCache(),
	}, nil
}

// Run 启动 MCP Server
func (s *Server) Run(ctx context.Context, transport string) error {
	// 创建 MCP Server 实例
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "ssh-mcp-server",
		Version: "1.0.0",
	}, nil)

	// 注册 exec 工具（SSH 连接参数作为工具参数）
	mcp.AddTool(server, &mcp.Tool{
		Name:        "exec",
		Description: "在远程 SSH 主机上执行命令（懒连接模式：使用缓存连接，TTL 90秒）",
	}, s.handleExec)

	// 注册 sudo_exec 工具
	mcp.AddTool(server, &mcp.Tool{
		Name:        "sudo_exec",
		Description: "在远程 SSH 主机上以 sudo 权限执行命令（懒连接模式：使用缓存连接，TTL 90秒）",
	}, s.handleSudoExec)

	// 根据传输方式启动服务
	switch transport {
	case "stdio":
		return server.Run(ctx, &mcp.StdioTransport{})
	default:
		return fmt.Errorf("不支持的传输方式: %s", transport)
	}
}

// execToolParams exec/sudo_exec 工具的参数（包含 SSH 连接参数和命令）
type execToolParams struct {
	Host       string `json:"host"        jsonschema:"SSH 目标主机地址，必选"`
	Username   string `json:"username"    jsonschema:"SSH 用户名，默认 root"`
	Password   string `json:"password"    jsonschema:"SSH 密码认证，与 private_key 二选一"`
	PrivateKey string `json:"private_key" jsonschema:"SSH 私钥文件路径，与 password 二选一"`
	Command    string `json:"command"     jsonschema:"要执行的命令，必选"`
}

// execOutput exec/sudo_exec 工具的输出结果
type execOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// validateExecToolParams 验证工具参数
func validateExecToolParams(params execToolParams) error {
	if params.Host == "" {
		return fmt.Errorf("host 参数不能为空")
	}
	if params.Command == "" {
		return fmt.Errorf("command 参数不能为空")
	}
	if params.Password == "" && params.PrivateKey == "" {
		return fmt.Errorf("必须提供 password 或 private_key 参数进行认证")
	}
	return nil
}

// toOptions 将工具参数转换为 SSH 连接参数
func toOptions(params execToolParams) Options {
	username := params.Username
	if username == "" {
		username = DefaultUsername()
	}
	return Options{
		Host:       params.Host,
		Username:   username,
		Password:   params.Password,
		PrivateKey: params.PrivateKey,
	}
}

// handleExec 处理 exec 工具调用
func (s *Server) handleExec(ctx context.Context, req *mcp.CallToolRequest, args execToolParams) (*mcp.CallToolResult, execOutput, error) {
	// 验证参数
	if err := validateExecToolParams(args); err != nil {
		return nil, execOutput{}, fmt.Errorf("参数验证失败: %w", err)
	}

	// 使用缓存创建 SSH 客户端
	opts := toOptions(args)
	client := NewClient(opts, s.cache)

	result, err := client.Exec(args.Command)
	if err != nil {
		return nil, execOutput{}, fmt.Errorf("执行命令失败: %w", err)
	}

	output := execOutput{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	}

	return nil, output, nil
}

// handleSudoExec 处理 sudo_exec 工具调用
func (s *Server) handleSudoExec(ctx context.Context, req *mcp.CallToolRequest, args execToolParams) (*mcp.CallToolResult, execOutput, error) {
	// 验证参数
	if err := validateExecToolParams(args); err != nil {
		return nil, execOutput{}, fmt.Errorf("参数验证失败: %w", err)
	}

	// 使用缓存创建 SSH 客户端
	opts := toOptions(args)
	client := NewClient(opts, s.cache)

	result, err := client.SudoExec(args.Command)
	if err != nil {
		return nil, execOutput{}, fmt.Errorf("执行 sudo 命令失败: %w", err)
	}

	output := execOutput{
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
		ExitCode: result.ExitCode,
	}

	return nil, output, nil
}
