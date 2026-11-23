# AlonaBot Skills 维护约定

本仓库发布给 AI 编码代理使用的 AlonaBot 开发技能。格式、版本与日期规则见 [README](README.md)「维护」，这里只列维护时必须遵守的事。

## 修改技能

- 内容以 AlonaBot 对应版本的源码为准：`docs/plugin-guide.zh-CN.md`、`docs/development.zh-CN.md`、`docs/agent-api.zh-CN.md`、`sdk/` 的导出 API 与 `contracts/proto/alonabot/v1/`。官方组件实现位于各自独立仓库。写进技能的 API、命令与限额都要在主仓库里核对过，尚未实现的能力明确写「尚未提供」。
- 技能、注释、文档用简体中文；标识符、命令和程序输出保留英文。`description` 写清触发任务，正文只保留会改变开发决策的信息。
- frontmatter 只用 Agent Skills 规范的键（`name`、`description`、`license`、`compatibility`、`metadata`、`allowed-tools`），自定义字段放进 `metadata`。为了不依赖 YAML 解析器，每个顶层键的值写在同一行，`metadata` 下每项缩进两个空格且值用双引号。
- 每个技能必须有 `agents/openai.yaml`，`default_prompt` 提到 `$<技能名>`。
- `SKILL.md` 正文少于 500 行；细节放 `references/`，只从 `SKILL.md` 链接一层。
- 新增或删除技能时同步 README 技能表与 `.claude-plugin/marketplace.json`。

## 版本与日期

- 不手改 `metadata.updated`、可见行和 README 表里的日期，用 `sh scripts/skills.sh stamp`；首次克隆后运行 `git config core.hooksPath .githooks`，提交时自动更新。
- `VERSION` 是适用的 AlonaBot 版本，按对应版本的实现核对后修改；维护者指定 main 为版本基准时，在 README 写明。同时改 marketplace 的 `metadata.version` 并运行 `stamp --all`，在 CHANGELOG 记一条。

## 完成前检查

```sh
sh scripts/skills.sh check --git
claude plugin validate .   # 有 Claude Code CLI 时
```

## Git

- 用户未明确要求时不 commit、不 push；`main` 的合并由维护者确认。
- 仓库不依赖 Go 或任何需要编译、安装的语言：工具只用 POSIX sh、awk、grep 与 git，写法要同时兼容 mawk、BSD awk 与 gawk，不用 `sed -i` 等 GNU 专有选项。
