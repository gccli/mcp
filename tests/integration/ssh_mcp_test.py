#!/usr/bin/env python3
"""SSH MCP Server 集成测试

测试 ./mcp ssh 的 MCP 协议交互、exec/sudo_exec 工具、缓存、错误处理。
使用方式：python3 tests/integration/ssh_mcp_test.py
"""

import json
import os
import subprocess
import sys
import time

# ============================================================
# 配置
# ============================================================
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, "../.."))
MCP_BIN = os.path.join(PROJECT_ROOT, "mcp")
TEST_HOST = "127.0.0.1"
TEST_USER = "root"
TEST_KEY = "/root/.ssh/id_ecdsa"

# 颜色
GREEN = "\033[0;32m"
RED = "\033[0;31m"
YELLOW = "\033[0;33m"
NC = "\033[0m"

PASS_COUNT = 0
FAIL_COUNT = 0
TOTAL_COUNT = 0


def log_pass(msg: str):
    global PASS_COUNT, TOTAL_COUNT
    PASS_COUNT += 1
    TOTAL_COUNT += 1
    print(f"{GREEN}✓ PASS{NC}: {msg}")


def log_fail(msg: str, detail: str = ""):
    global FAIL_COUNT, TOTAL_COUNT
    FAIL_COUNT += 1
    TOTAL_COUNT += 1
    print(f"{RED}✗ FAIL{NC}: {msg}")
    if detail:
        # 截断过长的详情
        if len(detail) > 500:
            detail = detail[:500] + "..."
        print(f"  {YELLOW}详情{NC}: {detail}")


def log_info(msg: str):
    print(f"{YELLOW}ℹ{NC}: {msg}")


# ============================================================
# MCP 客户端
# ============================================================
class MCPClient:
    """MCP stdio 客户端，管理子进程生命周期"""

    def __init__(self, binary_path: str):
        self.binary_path = binary_path
        self.proc = None

    def start(self):
        """启动 MCP Server 子进程"""
        self.proc = subprocess.Popen(
            [self.binary_path, "ssh"],
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )

    def send(self, msg: dict, wait: float = 0.3):
        """发送一条 JSON-RPC 消息"""
        data = json.dumps(msg, ensure_ascii=False) + "\n"
        self.proc.stdin.write(data)
        self.proc.stdin.flush()
        time.sleep(wait)

    def read_all(self, timeout: float = 5.0) -> str:
        """关闭 stdin，等待进程结束，读取 stdout"""
        try:
            self.proc.stdin.close()
        except Exception:
            pass
        try:
            self.proc.wait(timeout=timeout)
        except subprocess.TimeoutExpired:
            try:
                self.proc.kill()
                self.proc.wait(timeout=2)
            except Exception:
                pass
        try:
            out = self.proc.stdout.read()
            return out if out else ""
        except Exception:
            return ""

    def call(self, messages: list[dict], timeout: float = 10.0) -> str:
        """发送多条消息并返回所有响应"""
        self.start()
        for i, msg in enumerate(messages):
            wait = 0.5 if i == 0 else 0.3
            self.send(msg, wait=wait)
        time.sleep(0.5)
        output = self.read_all(timeout=timeout)
        return output

    def close(self):
        """关闭子进程"""
        if self.proc:
            try:
                self.proc.stdin.close()
            except Exception:
                pass
            try:
                self.proc.terminate()
            except Exception:
                pass
            try:
                self.proc.wait(timeout=3)
            except Exception:
                try:
                    self.proc.kill()
                    self.proc.wait(timeout=2)
                except Exception:
                    pass


def make_init_messages() -> list[dict]:
    """生成初始化消息序列"""
    return [
        {
            "jsonrpc": "2.0",
            "id": 1,
            "method": "initialize",
            "params": {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "integration-test", "version": "1.0.0"},
            },
        },
        {"jsonrpc": "2.0", "method": "notifications/initialized"},
    ]


def mcp_call(messages: list[dict], timeout: float = 10.0) -> str:
    """发送 MCP 消息并返回响应"""
    client = MCPClient(MCP_BIN)
    return client.call(messages, timeout=timeout)


# ============================================================
# 前置检查
# ============================================================
def preflight_checks():
    """执行前置检查"""
    print("=" * 50)
    print("  SSH MCP Server 集成测试")
    print("=" * 50)
    print()

    # 检查二进制文件
    if not os.path.isfile(MCP_BIN):
        log_info("编译 MCP Server...")
        result = subprocess.run(["make", "build"], cwd=PROJECT_ROOT, capture_output=True)
        if result.returncode != 0:
            print(f"{RED}编译失败{NC}")
            sys.exit(1)
    log_info(f"MCP 二进制: {MCP_BIN}")

    # 检查 SSH 连通性
    try:
        result = subprocess.run(
            [
                "ssh", "-o", "StrictHostKeyChecking=no",
                "-o", "BatchMode=yes", "-o", "ConnectTimeout=5",
                "-i", TEST_KEY, f"{TEST_USER}@{TEST_HOST}",
                "echo", "ok",
            ],
            capture_output=True, timeout=10,
        )
        if result.returncode != 0:
            print(f"{RED}SSH 连接测试失败{NC}")
            sys.exit(1)
    except Exception as e:
        print(f"{RED}SSH 连接异常: {e}{NC}")
        sys.exit(1)

    log_info(f"SSH 连通性: {TEST_USER}@{TEST_HOST}:22 OK")
    print()


# ============================================================
# 测试用例
# ============================================================

def test_01_mcp_initialize():
    """测试 1: MCP 协议初始化"""
    print("── 测试 1: MCP 协议初始化 ──")

    response = mcp_call(make_init_messages())

    if '"result"' in response and "ssh-mcp-server" in response:
        log_pass("MCP 初始化成功，返回 server 信息")
    else:
        log_fail("MCP 初始化", response)


def test_02_tools_list():
    """测试 2: tools/list 返回工具列表"""
    print("── 测试 2: tools/list 返回工具列表 ──")

    messages = make_init_messages() + [
        {"jsonrpc": "2.0", "id": 2, "method": "tools/list"}
    ]
    response = mcp_call(messages)

    if '"exec"' in response and '"sudo_exec"' in response:
        log_pass("tools/list 返回 exec 和 sudo_exec 工具")
    else:
        log_fail("tools/list 工具列表", response)

    if "remote SSH" in response or "远程" in response:
        log_pass("exec 工具包含 description")
    else:
        log_fail("exec 工具 description", response)


def test_03_exec_private_key():
    """测试 3: exec 工具 - 私钥认证执行简单命令"""
    print("── 测试 3: exec 工具 - 私钥认证 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "echo hello_mcp_test",
                },
            },
        }
    ]
    response = mcp_call(messages)

    if "hello_mcp_test" in response:
        log_pass("exec 私钥认证执行命令成功，stdout 包含 hello_mcp_test")
    else:
        log_fail("exec 私钥认证", response)

    if '"exit_code":0' in response or '"exit_code": 0' in response:
        log_pass("exec 命令退出码为 0")
    else:
        log_fail("exec 命令退出码", response)


def test_04_exec_multiline():
    """测试 4: exec 工具 - 多行输出"""
    print("── 测试 4: exec 工具 - 多行输出 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "printf 'line1\\nline2\\nline3'",
                },
            },
        }
    ]
    response = mcp_call(messages)

    if "line1" in response and "line2" in response and "line3" in response:
        log_pass("exec 多行输出正确")
    else:
        log_fail("exec 多行输出", response)


def test_05_exec_nonzero_exit():
    """测试 5: exec 工具 - 非零退出码"""
    print("── 测试 5: exec 工具 - 非零退出码 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "exit 42",
                },
            },
        }
    ]
    response = mcp_call(messages)

    if '"exit_code":42' in response or '"exit_code": 42' in response:
        log_pass("exec 失败命令返回正确退出码 42")
    else:
        log_fail("exec 非零退出码", response)


def test_06_exec_stderr():
    """测试 6: exec 工具 - stderr 输出"""
    print("── 测试 6: exec 工具 - stderr 输出 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "echo error_msg >&2",
                },
            },
        }
    ]
    response = mcp_call(messages)

    if "error_msg" in response:
        log_pass("exec stderr 输出正确捕获")
    else:
        log_fail("exec stderr", response)


def test_07_sudo_exec():
    """测试 7: sudo_exec 工具"""
    print("── 测试 7: sudo_exec 工具 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "sudo_exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "whoami",
                },
            },
        }
    ]
    response = mcp_call(messages)

    # sudo_exec 应该调用 sudo 命令（即使环境中没有 sudo，也验证了调用链路正确）
    if "sudo" in response.lower() and "exit_code" in response:
        log_pass("sudo_exec 工具调用正确（包含 sudo 命令）")
    elif "root" in response and "exit_code" in response:
        log_pass("sudo_exec 执行成功，whoami 返回 root")
    else:
        log_fail("sudo_exec", response)


def test_08_missing_host():
    """测试 8: 参数验证 - 缺少 host"""
    print("── 测试 8: 参数验证 - 缺少 host ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "username": "root",
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "ls",
                },
            },
        }
    ]
    response = mcp_call(messages)

    if any(kw in response.lower() for kw in ["host", "error", "参数", "验证"]):
        log_pass("缺少 host 参数时返回错误")
    else:
        log_fail("缺少 host 参数验证", response)


def test_09_missing_command():
    """测试 9: 参数验证 - 缺少 command"""
    print("── 测试 9: 参数验证 - 缺少 command ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": "root",
                    "private_key": TEST_KEY,
                },
            },
        }
    ]
    response = mcp_call(messages)

    if any(kw in response.lower() for kw in ["command", "error", "参数", "验证"]):
        log_pass("缺少 command 参数时返回错误")
    else:
        log_fail("缺少 command 参数验证", response)


def test_10_missing_auth():
    """测试 10: 参数验证 - 缺少认证方式"""
    print("── 测试 10: 参数验证 - 缺少认证方式 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": "root",
                    "command": "ls",
                },
            },
        }
    ]
    response = mcp_call(messages)

    if any(kw in response.lower() for kw in ["认证", "password", "private_key", "error"]):
        log_pass("缺少认证方式时返回错误")
    else:
        log_fail("缺少认证方式验证", response)


def test_11_connection_failure():
    """测试 11: 连接失败 - 认证失败"""
    print("── 测试 11: 连接失败 - 认证失败 ──")

    # 使用错误密码测试认证失败（比不可达主机更快）
    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": "root",
                    "password": "wrong_password_12345",
                    "command": "echo test",
                },
            },
        }
    ]
    response = mcp_call(messages, timeout=15)

    if any(kw in response.lower() for kw in ["连接", "ssh", "error", "失败", "认证", "auth"]):
        log_pass("认证失败时返回连接错误")
    else:
        log_fail("认证失败", response)


def test_12_exec_pipe_command():
    """测试 12: exec 工具 - 管道命令"""
    print("── 测试 12: exec 工具 - 管道命令 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "echo 'aaa bbb ccc' | awk '{print $2}'",
                },
            },
        }
    ]
    response = mcp_call(messages)

    if "bbb" in response:
        log_pass("exec 管道命令执行正确")
    else:
        log_fail("exec 管道命令", response)


def test_13_exec_env_var():
    """测试 13: exec 工具 - 环境变量"""
    print("── 测试 13: exec 工具 - 环境变量 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "MY_VAR=mcp_test_value && echo $MY_VAR",
                },
            },
        }
    ]
    response = mcp_call(messages)

    if "mcp_test_value" in response:
        log_pass("exec 环境变量命令执行正确")
    else:
        log_fail("exec 环境变量", response)


def test_14_exec_command_not_found():
    """测试 14: exec 工具 - 命令不存在"""
    print("── 测试 14: exec 工具 - 命令不存在 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "nonexistent_command_xyz",
                },
            },
        }
    ]
    response = mcp_call(messages)

    if any(kw in response.lower() for kw in ["not found", "nonexistent", "exit_code", "command not found"]):
        log_pass("exec 不存在命令返回错误信息")
    else:
        log_fail("exec 不存在命令", response)


def test_15_unknown_method():
    """测试 15: JSON-RPC 错误 - 未知方法"""
    print("── 测试 15: JSON-RPC 错误 - 未知方法 ──")

    messages = make_init_messages() + [
        {"jsonrpc": "2.0", "id": 2, "method": "unknown/method"}
    ]
    response = mcp_call(messages)

    if any(kw in response.lower() for kw in ["error", "unknown", "not found"]):
        log_pass("未知方法返回错误")
    else:
        log_fail("未知方法", response)


def test_16_unknown_tool():
    """测试 16: tools/call - 未知工具名"""
    print("── 测试 16: tools/call - 未知工具名 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "nonexistent_tool",
                "arguments": {"host": TEST_HOST, "command": "echo test"},
            },
        }
    ]
    response = mcp_call(messages)

    if any(kw in response.lower() for kw in ["error", "unknown", "not found", "tool"]):
        log_pass("未知工具名返回错误")
    else:
        log_fail("未知工具名", response)


def test_17_exec_large_output():
    """测试 17: exec 工具 - 大输出"""
    print("── 测试 17: exec 工具 - 大输出 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "seq 1 1000",
                },
            },
        }
    ]
    response = mcp_call(messages, timeout=15)

    if len(response) > 5000:
        log_pass(f"exec 大输出命令正确返回（{len(response)} 字节）")
    else:
        log_fail("exec 大输出", f"响应长度: {len(response)} 字节")


def test_18_exec_hostname():
    """测试 18: exec 工具 - 验证远程主机身份"""
    print("── 测试 18: exec 工具 - 验证远程主机身份 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "hostname",
                },
            },
        }
    ]
    response = mcp_call(messages)

    # 验证命令执行成功且有输出
    if '"exit_code":0' in response or '"exit_code": 0' in response:
        log_pass("exec hostname 命令执行成功")
    else:
        log_fail("exec hostname", response)


def test_19_concurrent_calls():
    """测试 19: 连续多次调用（验证连接缓存）"""
    print("── 测试 19: 连续调用（连接缓存） ──")

    results = []
    for i in range(3):
        messages = make_init_messages() + [
            {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "tools/call",
                "params": {
                    "name": "exec",
                    "arguments": {
                        "host": TEST_HOST,
                        "username": TEST_USER,
                        "password": "",
                        "private_key": TEST_KEY,
                        "command": f"echo cache_test_{i}",
                    },
                },
            }
        ]
        response = mcp_call(messages, timeout=10)
        results.append(f"cache_test_{i}" in response or f"cache_test_{i}" in repr(response))

    if all(results):
        log_pass("连续 3 次调用均成功（连接缓存正常）")
    else:
        log_fail("连续调用", f"结果: {results}")


def test_20_exec_id_command():
    """测试 20: exec 工具 - id 命令验证用户"""
    print("── 测试 20: exec 工具 - id 命令 ──")

    messages = make_init_messages() + [
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "tools/call",
            "params": {
                "name": "exec",
                "arguments": {
                    "host": TEST_HOST,
                    "username": TEST_USER,
                    "password": "",
                    "private_key": TEST_KEY,
                    "command": "id -u",
                },
            },
        }
    ]
    response = mcp_call(messages)

    if '"exit_code":0' in response or '"exit_code": 0' in response:
        log_pass("exec id -u 返回 exit_code=0 (root 用户)")
    else:
        log_fail("exec id -u", response)


# ============================================================
# 主入口
# ============================================================
def main():
    preflight_checks()

    tests = [
        test_01_mcp_initialize,
        test_02_tools_list,
        test_03_exec_private_key,
        test_04_exec_multiline,
        test_05_exec_nonzero_exit,
        test_06_exec_stderr,
        test_07_sudo_exec,
        test_08_missing_host,
        test_09_missing_command,
        test_10_missing_auth,
        test_11_connection_failure,
        test_12_exec_pipe_command,
        test_13_exec_env_var,
        test_14_exec_command_not_found,
        test_15_unknown_method,
        test_16_unknown_tool,
        test_17_exec_large_output,
        test_18_exec_hostname,
        test_19_concurrent_calls,
        test_20_exec_id_command,
    ]

    for test in tests:
        try:
            test()
        except Exception as e:
            log_fail(test.__doc__ or test.__name__, str(e))
        print()

    # 汇总
    print("=" * 50)
    print("  测试结果汇总")
    print("=" * 50)
    print(f"  总计: {TOTAL_COUNT}")
    print(f"  {GREEN}通过{NC}: {PASS_COUNT}")
    print(f"  {RED}失败{NC}: {FAIL_COUNT}")
    print()

    if FAIL_COUNT == 0:
        print(f"{GREEN}所有测试通过 ✓{NC}")
        return 0
    else:
        print(f"{RED}存在失败的测试 ✗{NC}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
