# Quality Guidelines

> Code quality standards for backend development.

---

## Overview

<!--
Document your project's quality standards here.

Questions to answer:
- What patterns are forbidden?
- What linting rules do you enforce?
- What are your testing requirements?
- What code review standards apply?
-->

## Forbidden Patterns

<!-- Patterns that should never be used and why -->

(To be filled by the team)

---

## Required Patterns

- **TDD (Test-Driven Development)**: 新功能、重构或 Bug 修复必须遵循「先写失败测试 → 实现最小代码让测试通过 → 重构」的红绿循环。
  - 不允许先实现逻辑再补测试（除非是历史遗留代码的补测）。
  - 提交前必须确保所有测试通过（`go test ./...` 全绿）。
- **Table-Driven Tests**: 优先使用表驱动测试组织多场景用例，保持测试结构统一。
- ** testify 断言库**: 使用 `stretchr/testify` 的 `assert` / `require` 替代手写 `if err != nil { t.Fatal(...) }`，提升可读性。

---

## Testing Requirements

- **覆盖率底线**: 新代码的行覆盖率不得低于 **80%**；核心业务路径、错误分支必须覆盖。
- **测试先行**: 每个功能 PR 必须包含对应测试文件（`*_test.go`），否则禁止合并。
- **边界与异常**: 测试必须覆盖边界值（空值、零值、最大值）和错误路径，不能仅测试 Happy Path。
- **Mock 与隔离**: 涉及外部依赖（网络、数据库、文件系统）时，使用接口 + Mock 进行单元测试隔离；集成测试放在独立文件并标注 `//go:build integration`。
- **Race 检测**: CI 中运行 `go test -race ./...`，任何 Data Race 必须修复后才能合并。

---

## Code Review Checklist

<!-- What reviewers should check -->

- [ ] 是否有对应测试？测试是否在实现之前提交？
- [ ] 测试是否覆盖了错误路径和边界条件？
- [ ] `go test ./...` 是否全部通过？
- [ ] `go test -race ./...` 是否无 Race 告警？
- [ ] 是否使用了 testify 而非手写冗长断言？
- [ ] 是否避免了为不可测试代码找借口的「跳过测试」注释？
