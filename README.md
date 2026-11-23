# AlonaBot Skills

[简体中文](README.zh-CN.md)

Plugin development, platform integration, and protocol troubleshooting skills for AlonaBot **v0.3.0**, checked against the core repository’s main branch. The skills share the Agent Skills `SKILL.md` format and include OpenAI Codex metadata and an Anthropic Claude Code marketplace entry. Skill instructions are written in Simplified Chinese.

| Skill | Purpose | Last updated |
| --- | --- | --- |
| [alonabot-plugin](skills/alonabot-plugin/SKILL.md) | Plugins and Agent extensions: commands, messages, services, tasks, configuration, storage, testing, and packaging | 2026-11-23 |
| [alonabot-adapter](skills/alonabot-adapter/SKILL.md) | Platform integration: instances, events, cursors, media, actions, and account setup pages | 2026-11-23 |
| [alonabot-wire-protocol](skills/alonabot-wire-protocol/SKILL.md) | Troubleshoot session errors during handshake, preparation, activation, and calls | 2026-11-23 |

## OpenAI Codex

Use `$skill-installer` in Codex and ask it to install the `skills/<skill-name>` directories listed above from `https://github.com/AlonaBot/skills`.

For manual installation, run these fish commands to copy the skills into your personal directory. For a project installation, use the project’s `.agents/skills/` directory instead:

```fish
git clone https://github.com/AlonaBot/skills.git ~/src/alonabot-skills
mkdir -p ~/.agents/skills
cp -R ~/src/alonabot-skills/skills/alonabot-* ~/.agents/skills/
```

Invoke `$alonabot-plugin`, `$alonabot-adapter`, or `$alonabot-wire-protocol`, or let Codex select a skill based on the task. Restart Codex if a newly installed skill does not appear. See the [Codex documentation](https://developers.openai.com/codex/skills/).

## Anthropic Claude Code

Install through the plugin marketplace:

```text
/plugin marketplace add AlonaBot/skills
/plugin install alonabot-dev@alonabot-skills
```

Invoke `/alonabot-dev:alonabot-plugin` or another skill command, or let Claude select a skill based on its description.

For manual installation, copy the skill directories under `skills/` into `~/.claude/skills/` for personal use or the project’s `.claude/skills/` directory. Manually installed skills use commands such as `/alonabot-plugin`. See the [Claude Code documentation](https://code.claude.com/docs/en/skills).

## Claude.ai and Claude API

For Claude.ai, zip one skill directory and upload it under Skills in settings. Keep the top-level skill directory in the archive, such as `alonabot-plugin/SKILL.md` together with its `references/` and `agents/` directories. To package all three skills, run from the repository root:

```fish
mkdir -p bin
cd skills
for name in alonabot-plugin alonabot-adapter alonabot-wire-protocol
    zip -r ../bin/$name.zip $name
end
cd ..
```

For the Claude API, upload one skill’s files through the Skills API, keeping a shared top-level directory. Reference the returned skill ID and version in Messages under `container.skills` with `type: "custom"`, and enable code execution. See the [Skills API](https://platform.claude.com/docs/en/api/beta/skills/create) and [integration guide](https://platform.claude.com/docs/en/agents-and-tools/agent-skills/quickstart) for parameters. Provide the source code needed for development in the execution environment.

## Maintenance

Skills use `name`, `description`, and `compatibility`, with version and date stored as strings in `metadata`. Each skill’s `agents/openai.yaml` supplies Codex display metadata. Task-specific details live in `references/` and are read as needed.

`VERSION` records the supported AlonaBot version. For an update, check the SDK and documentation, update VERSION and the marketplace version, and run the date tool. Record changes in [CHANGELOG](CHANGELOG.md). Keep both README versions in sync; the date tool updates both skill tables.

```fish
sh scripts/skills.sh stamp --all
sh scripts/skills.sh check --git
claude plugin validate .
```

Checks cover skill metadata, versions and dates, relative links, Codex metadata, the Claude marketplace manifest, and both README skill tables. See [AGENTS.md](AGENTS.md) for repository maintenance rules.
