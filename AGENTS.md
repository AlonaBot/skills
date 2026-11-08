# AlonaBot Skills 维护约定

本仓库发布给 AI 编码代理使用的 AlonaBot 开发技能。格式、版本与日期规则见 [README](README.md)「格式约定」「版本与日期」，这里只列维护时必须遵守的事。

## 修改技能

- 内容以 AlonaBot 主仓库为准：`spec/CONTRACTS-v0.1.md`、`docs/plugin-guide.md`、`sdk/` 的导出 API 与 `ARCHITECTURE.md` 的状态列。写进技能的 API、命令与限额都要在主仓库里核对过，尚未实现的能力明确写「尚未提供」。
- 技能、注释、文档用简体中文；标识符、命令和程序输出保留英文。`description` 写清何时使用、何时改用别的技能，可以带少量英文关键词帮助匹配。
- frontmatter 只用 Agent Skills 规范的键（`name`、`description`、`license`、`compatibility`、`metadata`、`allowed-tools`），自定义字段放进 `metadata` 且值为字符串。
- 每个技能必须有 `agents/openai.yaml`，`default_prompt` 提到 `$<技能名>`。
- `SKILL.md` 正文少于 500 行；细节放 `references/`，只从 `SKILL.md` 链接一层。
- 新增或删除技能时同步 README 技能表与 `.claude-plugin/marketplace.json`。

## 版本与日期

- 不手改 `metadata.updated`、可见行和 README 表里的日期，用 `go run ./tools/skillcheck stamp`；首次克隆后运行 `git config core.hooksPath .githooks`，提交时自动更新。
- `VERSION` 是适用的 AlonaBot 版本，只在 AlonaBot 发布新版本、并已按新版本核对过技能内容时修改；同时改 marketplace 的 `metadata.version` 并运行 `stamp --all`，在 CHANGELOG 记一条。

## 完成前检查

```sh
go vet ./...
go run ./tools/skillcheck check --git
claude plugin validate .   # 有 Claude Code CLI 时
```

## Git

- 用户未明确要求时不 commit、不 push；`main` 的合并由维护者确认。
- 检查工具只用标准库与 `go.yaml.in/yaml/v3`；新依赖先征得维护者同意。
