# 更新记录

按适用的 AlonaBot 版本分组，组内按日期倒序。

## 0.3.0

### 2026-11-23

- 按 AlonaBot main 更新插件对象入口、构建资源、配置、SDK 消息类型、Service、任务、KV 与测试打包流程。
- 更新 Adapter 类型、可选接口和独立组件仓库位置，删除旧规格路径及未实现状态。
- 保留 Codex 与 Claude Code 安装入口，补全 Claude.ai 打包和 Claude API 接入说明。

## 0.1-beta

### 2026-11-09

- 去掉对 Go 的依赖：检查与日期工具改为 `scripts/skills.sh`（POSIX sh + awk），删除 `go.mod` 与 `tools/skillcheck`。

### 2026-11-08

- 初始版本：`alonabot-plugin`、`alonabot-adapter`、`alonabot-wire-protocol` 三个技能。
- 同时符合 Agent Skills 规范与 Codex 技能格式；提供 Claude Code 插件市场清单。
- `tools/skillcheck` 校验格式并维护版本与更新日期；提交钩子与 CI 接入。
