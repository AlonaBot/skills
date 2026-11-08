# AlonaBot Skills

给 AI 编码代理用的 AlonaBot 开发技能：编写插件、接入平台 Adapter、排查 Wire 协议。同一份文件同时符合 [Agent Skills 规范](https://agentskills.io/specification)（Claude Code、Claude.ai、Claude API）与 [OpenAI Codex 技能](https://developers.openai.com/codex/skills)。

**适用 AlonaBot 版本：0.1-beta**（见 [VERSION](VERSION)）。框架本身的维护技能在 AlonaBot 主仓库的 `.agents/skills/`，不在这里。

## 技能

| 技能 | 用途 | 最后更新 |
| --- | --- | --- |
| [alonabot-plugin](skills/alonabot-plugin/SKILL.md) | 用 Go SDK 编写 Native 插件：注册、配置、存储、生成与本地运行 | 2026-11-08 |
| [alonabot-adapter](skills/alonabot-adapter/SKILL.md) | 接入新的聊天平台：实例、事件交付、游标、能力声明与模拟服务测试 | 2026-11-08 |
| [alonabot-wire-protocol](skills/alonabot-wire-protocol/SKILL.md) | 把会话失败、错误码与超时对应到协议阶段并找出原因 | 2026-11-08 |

每个技能的 `SKILL.md` 标题下都有一行 `> AlonaBot 0.1-beta · 最后更新 YYYY-MM-DD`，与 frontmatter 的 `metadata` 保持一致。

## 安装

技能需要读取 AlonaBot 源码（目前是私有仓库），请在能访问 `github.com/AlonaBot/AlonaBot` 的环境中使用。

**Claude Code**（插件市场，推荐）：

```text
/plugin marketplace add AlonaBot/skills
/plugin install alonabot-dev@alonabot-skills
```

**Claude Code**（手动）：把 `skills/` 下的目录复制或链接到 `~/.claude/skills/`（个人）或插件工程的 `.claude/skills/`（项目）。

**Codex**：复制或链接到 `~/.agents/skills/`（个人）或插件工程的 `.agents/skills/`（项目），重启 Codex 后用 `$alonabot-plugin` 等名字显式调用，或让 Codex 按描述自动选择。

```sh
git clone https://github.com/AlonaBot/skills.git ~/src/alonabot-skills
mkdir -p ~/.claude/skills ~/.agents/skills
for s in ~/src/alonabot-skills/skills/*/; do
  ln -s "$s" ~/.claude/skills/
  ln -s "$s" ~/.agents/skills/
done
```

**Claude.ai / Claude API**：把单个技能目录打成 zip 上传。

## 格式约定

一个技能目录同时满足两家的要求：

```text
skills/<name>/
├── SKILL.md            # 两家共用：YAML frontmatter + Markdown 正文
├── agents/openai.yaml  # Codex 的界面信息（显示名、简述、默认提示词）
└── references/         # 可选：按需读取的详细资料，只从 SKILL.md 链接一层
```

SKILL.md 的 frontmatter 只使用 Agent Skills 规范定义的键，自定义字段放在 `metadata`（字符串到字符串的映射）里，避免被严格的客户端拒绝：

```yaml
---
name: alonabot-plugin            # 与目录同名；小写字母、数字、单个连字符，≤64；不含 claude/anthropic
description: 何时使用、何时不用   # 1–1024 字符，不含 < >
compatibility: AlonaBot 0.1-beta；Go 1.27+
metadata:
  alonabot-version: "0.1-beta"   # 必须等于 VERSION
  updated: "2026-11-08"          # 最后修改日期，由工具维护
---
```

正文控制在 500 行以内，详细资料放进 `references/`。

## 版本与日期

- **版本对应。** `VERSION` 是唯一来源，表示这批技能针对的 AlonaBot 版本。每个技能的 `metadata.alonabot-version`、可见行和 `.claude-plugin/marketplace.json` 的 `metadata.version` 都必须与它一致。AlonaBot 发布新版本时：改 `VERSION` 与 marketplace 版本 → 按新版本核对并修改技能内容 → `go run ./tools/skillcheck stamp --all` → 在 [CHANGELOG](CHANGELOG.md) 记录 → 打 tag `v<VERSION>`。
- **日期自动更新。** 启用提交钩子后，每次提交会把有改动的技能的 `metadata.updated` 和全部可见行改成当天日期：

  ```sh
  git config core.hooksPath .githooks
  ```

  没有钩子时手动运行 `go run ./tools/skillcheck stamp <技能名>`。CI 运行 `check --git`，技能被改动而日期早于该次提交时失败，所以日期不会悄悄过期。

## 检查

```sh
go run ./tools/skillcheck check        # frontmatter、命名、描述长度、版本、可见行、链接、openai.yaml、marketplace
go run ./tools/skillcheck check --git  # 另核对更新日期不早于最后一次修改该技能的提交
```

维护本仓库的约定见 [AGENTS.md](AGENTS.md)。
