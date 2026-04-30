# API Design
- SSH MCP Server应采用懒连接模式，不在启动时连接，将host、username、password、private-key等连接参数作为exec和sudo_exec操作的参数而非全局配置。

# Testing
- 开发时必须遵循TDD（测试驱动开发）原则，先编写测试用例，再进行功能开发。
