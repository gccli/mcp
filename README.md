# MCP Server

一个基于 Go 实现的 [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) Server，目前提供 **SSH MCP Server**，支持通过 SSH 在远程主机上执行命令。

## 功能特性

- **SSH 远程执行**：支持 `exec` 和 `sudo_exec` 两个 MCP 工具
- **懒连接模式**：Server 启动时不连接任何主机，仅在工具调用时才建立 SSH 连接
- **连接缓存**：SSH 连接缓存（TTL 90 秒），减少重复连接开销
- **双认证方式**：支持密码认证和私钥认证
- **stdio 传输**：基于标准输入输出与 MCP Client 通信

## 快速开始

### 构建

```bash
make build
```

### 运行 SSH MCP Server

```bash
./mcp ssh
```

Server 将以 `stdio` 模式启动，等待 MCP Client 通过标准输入输出发送 JSON-RPC 消息。

## 如何 Inspect / 调试 MCP Server

由于 MCP Server 通常以 **stdio** 模式运行（通过标准输入输出进行 JSON-RPC 通信），直接与 Server 交互需要特定的工具和方法。以下是几种常用的 inspect 和调试方式：

### 方式一：使用官方 MCP Inspector（推荐）

MCP 官方提供了可视化 Inspector 工具，可以方便地查看 Server 提供的 Tools、Resources、Prompts，并直接发起调用。

#### 1. 安装并运行 Inspector

```bash
# 使用 npx 直接运行（无需安装）
npx @modelcontextprotocol/inspector ./mcp ssh
```

运行后，Inspector 会：
1. 启动你的 MCP Server（`./mcp ssh`）
2. 自动打开浏览器界面（默认地址通常是 `http://localhost:5173`）
3. 在界面中展示可用的 Tools、Resources 等信息

#### 2. 在 Inspector 中测试工具调用

在 Inspector 的 **Tools** 标签页中：
- 可以看到 `exec` 和 `sudo_exec` 两个工具
- 点击工具名称，填写参数（host、username、password/private_key、command）
- 点击 **Run Tool** 执行，查看返回结果

### 方式二：使用 Claude Desktop 集成测试

如果你使用 Claude Desktop，可以将 MCP Server 配置到 Claude Desktop 中进行实际测试。

**macOS 配置路径**：
```
~/Library/Application Support/Claude/claude_desktop_config.json
```

**配置示例**：
```json
{
  "mcpServers": {
    "ssh": {
      "command": "/absolute/path/to/mcp",
      "args": ["ssh"]
    }
  }
}
```

配置完成后重启 Claude Desktop，在对话中即可调用 `exec` 或 `sudo_exec` 工具。

### 方式三：手动通过 stdio 发送 JSON-RPC 消息

如果你需要底层调试，可以直接向 Server 的标准输入发送 JSON-RPC 消息。

#### 1. 启动 Server 并发送初始化请求

```bash
# 在一个终端启动 Server
./mcp ssh

# 在另一个终端，向该进程发送 JSON-RPC 消息（通过管道）
```

#### 2. JSON-RPC 消息格式示例

**初始化请求**（必须首先发送）：
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "initialize",
  "params": {
    "protocolVersion": "2024-11-05",
    "capabilities": {},
    "clientInfo": {
      "name": "test-client",
      "version": "1.0.0"
    }
  }
}
```

发送后需要紧接着发送 **initialized 通知**（无 id）：
```json
{
  "jsonrpc": "2.0",
  "method": "notifications/initialized"
}
```

**列出可用工具**：
```json
{
  "jsonrpc": "2.0",
  "id": 2,
  "method": "tools/list"
}
```

**调用 exec 工具**：
```json
{
  "jsonrpc": "2.0",
  "id": 3,
  "method": "tools/call",
  "params": {
    "name": "exec",
    "arguments": {
      "host": "192.168.1.100",
      "username": "root",
      "password": "your-password",
      "command": "uname -a"
    }
  }
}
```

> **注意**：每条 JSON-RPC 消息必须以换行符（`\n`）分隔。stdio 模式下 Server 会持续从标准输入读取行，解析为 JSON-RPC 请求。

#### 3. 使用脚本批量测试

你也可以用脚本快速测试：

```bash
# 将请求写入文件，然后通过管道发送
cat <<'EOF' | ./mcp ssh
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
EOF
```

### 方式四：查看日志输出

MCP Server 在运行时会输出日志到标准错误输出（stderr），你可以通过重定向查看运行状态：

```bash
# 将 stderr 重定向到日志文件
./mcp ssh 2> server.log
```

或者在另一个终端实时查看：

```bash
tail -f server.log
```

## 项目结构

```
.
├── cmd/ssh/          # SSH 子命令（Cobra CLI）
├── internal/ssh/     # SSH MCP Server 核心实现
│   ├── server.go     # MCP Server 注册和运行
│   ├── client.go     # SSH 客户端（命令执行）
│   ├── cache.go      # SSH 连接缓存
│   └── options.go    # SSH 连接配置选项
├── main.go           # 程序入口
├── go.mod            # Go 模块定义
└── Makefile          # 构建脚本
```

## 测试

```bash
make test
```

## 许可证

MIT
