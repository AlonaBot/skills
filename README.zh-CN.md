# AlonaBot Skills

[English](README.md)

面向 AlonaBot **v0.3.0** 的插件开发、平台接入与协议排查技能，按主仓库 main 核对。共用 Agent Skills 的 `SKILL.md` 格式，提供 OpenAI Codex 元数据与 Anthropic Claude Code 插件市场入口。技能正文使用简体中文。

| 技能 | 用途 | 最后更新 |
| --- | --- | --- |
| [alonabot-plugin](skills/alonabot-plugin/SKILL.md) | 插件与 Agent 扩展：命令、消息、Service、任务、配置、存储、测试与打包 | 2026-11-23 |
| [alonabot-adapter](skills/alonabot-adapter/SKILL.md) | 平台接入：实例、事件、游标、媒体、动作与账号配置页 | 2026-11-23 |
| [alonabot-wire-protocol](skills/alonabot-wire-protocol/SKILL.md) | 按握手、准备、激活与调用阶段排查会话错误 | 2026-11-23 |

## OpenAI Codex

在 Codex 中调用 `$skill-installer`，指定从 `https://github.com/AlonaBot/skills` 安装上述 `skills/<技能名>` 目录。

也可手动安装。以下为 fish 命令，把技能复制到个人目录；项目安装时将目标改为工程的 `.agents/skills/`：

```fish
git clone https://github.com/AlonaBot/skills.git ~/src/alonabot-skills
mkdir -p ~/.agents/skills
cp -R ~/src/alonabot-skills/skills/alonabot-* ~/.agents/skills/
```

以 `$alonabot-plugin`、`$alonabot-adapter` 或 `$alonabot-wire-protocol` 调用，也可按任务自动选择。新安装后若未出现，重启 Codex。[Codex 官方说明](https://developers.openai.com/codex/skills/)。

## Anthropic Claude Code

通过插件市场安装：

```text
/plugin marketplace add AlonaBot/skills
/plugin install alonabot-dev@alonabot-skills
```

调用 `/alonabot-dev:alonabot-plugin` 等技能命令，或让 Claude 按描述自动选择。

手动安装时，将 `skills/` 下的技能目录复制到 `~/.claude/skills/`（个人）或工程的 `.claude/skills/`（项目）。手动安装后使用 `/alonabot-plugin` 等命令。[Claude Code 官方说明](https://code.claude.com/docs/en/skills)。

## Claude.ai 与 Claude API

Claude.ai：将一个技能目录压缩为 zip，在设置中的 Skills 上传。zip 内保留顶层技能目录，例如 `alonabot-plugin/SKILL.md` 及其 `references/`、`agents/`。在仓库根目录打包：

```fish
mkdir -p bin
cd skills
for name in alonabot-plugin alonabot-adapter alonabot-wire-protocol
    zip -r ../bin/$name.zip $name
end
cd ..
```

Claude API：通过 Skills API 上传单个技能的文件集合，保留共同的顶层目录；取得 skill ID 后，在 Messages 的 `container.skills` 中以 `type: "custom"`、`skill_id` 和版本引用，并启用 code execution。上传与调用参数见 [Skills API](https://platform.claude.com/docs/en/api/beta/skills/create) 与 [接入指南](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/quickstart)。API 使用时将开发所需源码提供到执行环境。

## 维护

技能使用 `name`、`description` 与 `compatibility`，版本和日期置于字符串类型的 `metadata`。每个技能的 `agents/openai.yaml` 提供 Codex 显示信息；任务细节按需放在 `references/`。

`VERSION` 记录适用的 AlonaBot 版本。升级时核对 SDK 与文档、更新 VERSION 和 marketplace 版本，再运行日期工具；变更记录见 [CHANGELOG](CHANGELOG.md)。中英文 README 同步维护，日期工具同时更新两份技能表。

```fish
sh scripts/skills.sh stamp --all
sh scripts/skills.sh check --git
claude plugin validate .
```

检查覆盖技能元数据、版本日期、相对链接、Codex 元数据、Claude 市场清单与两份 README 技能表。维护约定见 [AGENTS.md](AGENTS.md)。
