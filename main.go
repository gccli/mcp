package main

import (
	"fmt"
	"os"

	"mcp/cmd/ssh"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "mcp",
	Short: "MCP Server - 支持多种类型的 MCP Server",
	Long:  "MCP Server 是一个支持多种类型 Server 的命令行工具，通过子命令启动不同种类的 MCP Server。",
}

func init() {
	rootCmd.PersistentFlags().String("transport", "stdio", "MCP Server 传输方式 (stdio)")
	rootCmd.AddCommand(ssh.Cmd())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
