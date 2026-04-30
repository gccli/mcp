package ssh

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mcp/internal/ssh"

	"github.com/spf13/cobra"
)

// Cmd 返回 ssh 子命令
func Cmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ssh",
		Short: "启动 SSH MCP Server",
		Long:  "启动一个 SSH MCP Server，提供远程命令执行能力。\n支持 exec 和 sudo_exec 两个工具。\n\nSSH 连接参数（host、username、password/private_key）作为工具调用参数传入，\nServer 启动时不连接任何主机，仅在工具调用时才建立连接（懒连接模式）。",
		RunE:  runSSH,
	}
}

func runSSH(cmd *cobra.Command, args []string) error {
	transport, _ := cmd.Flags().GetString("transport")

	if transport != "stdio" {
		return fmt.Errorf("目前仅支持 stdio 传输方式")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	server, err := ssh.NewServer()
	if err != nil {
		return fmt.Errorf("创建 SSH MCP Server 失败: %w", err)
	}

	log.Printf("启动 SSH MCP Server (transport=%s)", transport)

	if err := server.Run(ctx, transport); err != nil {
		return fmt.Errorf("运行 SSH MCP Server 失败: %w", err)
	}

	return nil
}
