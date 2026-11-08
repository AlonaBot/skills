---
name: alonabot-plugin
description: 使用 AlonaBot Go SDK 编写、修改或排查 Native Go 插件时使用，包括链式注册命令、文本匹配、事件、Service、Event Bus 订阅与 HTTP 路由，编写 plugin.yaml 与 config_schema.json，读取配置与 secret，使用受管 SQLite，以及运行 alonabot plugin generate、plugin check 与 alonabot run。Use when building an AlonaBot plugin with the sdk package. 编写平台 Adapter 时改用 alonabot-adapter；修改 AlonaBot 框架本身时使用 AlonaBot 仓库内的维护技能。
compatibility: AlonaBot 0.1-beta；Go 1.27+；需要能读取 github.com/AlonaBot/AlonaBot 源码。
metadata:
  alonabot-version: "0.1-beta"
  updated: "2026-11-08"
---

# 编写 AlonaBot 插件

> AlonaBot 0.1-beta · 最后更新 2026-11-08

插件是独立的 Native Go 进程，只经 `github.com/AlonaBot/AlonaBot/sdk` 与 Core 通信。本技能给出 0.1-beta 的写法与边界；行为的权威是 AlonaBot 仓库的 `spec/CONTRACTS-v0.1.md`，插件作者规则是 `docs/plugin-guide.md`，两者与本技能冲突时以仓库为准，并提醒用户更新本技能。

## 先确认版本与现状

1. 读取插件工程 `go.mod` 中 `github.com/AlonaBot/AlonaBot` 的版本或 replace 路径；不是 0.1-beta 时告诉用户本技能可能过期。
2. 能访问 AlonaBot 源码时，以 `sdk/` 的导出 API、`examples/plugins/` 与 `ARCHITECTURE.md` 的「状态」列为准，不凭记忆补 API。
3. 0.1-beta 仍在实施：定时任务（`Schedules`）会让 `Run` 返回 UNSUPPORTED，`plugintest` 测试套件、`alonabot build`/`pack`/`install` 尚未提供，私有 HTTP 路由一律 401。用户需要这些能力时如实说明，不写假实现。

## 工程结构

```text
myplugin/
├── go.mod                # 自己管理依赖，与 Core 和其他插件互不影响
├── main.go               # 根插件与 main
├── plugin.yaml           # 作者信息：id、name、version、source（必填）
├── config_schema.json    # 可选，配置声明
├── i18n/en-US.json       # 用户可见文案，至少 en-US
└── alonabot_gen.go       # 由 alonabot plugin generate 生成，不手改
```

`plugin.yaml` 只写作者信息：必填 `id`、`name`、SemVer `version`、`source`，可选 `description`、`author`、`role`、`icon`、`adapter_dependencies`、`plugin_dependencies`，写框架外数据时加 `prepare_effects: external`。`manifest_version`、`entrypoint`、`os/arch`、`migration_mode`、协议版本等由构建补上，作者写了 `plugin check` 会报错。ID 规则与市场前缀 `alonabot_plugin_` 见 [references/sdk-api.md](references/sdk-api.md#manifest-与-id)。

## 根插件

根类型以值嵌入 `sdk.Base`，只覆盖用到的 Hook，`main` 调用 `sdk.Run`：

```go
type demo struct{ sdk.Base }

func (p *demo) Commands(r *sdk.CommandRegistry) {
	r.Command("hello").Handle(p.hello)
}

func (p *demo) hello(c *sdk.Context) (message.Chain, error) {
	return message.Chain{message.Text("hello")}, nil
}

func main() {
	if err := sdk.Run(&demo{}); err != nil {
		slog.Error("demo plugin stopped with an error", "error", err)
		os.Exit(1)
	}
}
```

- 生命周期：`OnConfigLoad`（纯配置变换）→ `OnStart`（打开资源，不能发起 Send、Service 或 Publish）→ `OnReady`（激活后执行）→ 业务 → `OnStop`（在期限内关闭资源，结束后台 goroutine）。
- 注册方法 `Commands`、`Events`、`Services`、`Subscriptions`、`HTTP` 各在 Collect 时调用一次。每条链必须以 `Handle` 结束；漏写、重复调用或在 `Handle` 之后继续调用都会让 Collect 报错。
- 两种 Handler：返回 `error` 的手动型，用 `c.Send` 发送；返回 `(message.Chain, error)` 的自动回复型，非空 Chain 由 Core 在 Handler 返回后、本次显式发送之后发送一次。

各注册入口、Handler 参数绑定、权限与发送的签名见 [references/sdk-api.md](references/sdk-api.md)。

## 必须遵守的规则

- **stdout 是协议通道。** 不往 stdout 打印；用 `log/slog` 或 `c.Logger()`，SDK 把记录交给 Core。
- **文案分层。** 日志、错误字符串与 safe_message 用英文；给用户看的文字放 `i18n/<locale>.json` 的 `messages`，至少 en-US。
- **不泄露 secret。** 不记录 token、完整配置与 DSN；secret 只在 `config_schema.json` 中以 `"secret": true` 声明，不写进日志、错误和业务数据库。
- **依赖从简。** 只导入用到的包，能用标准库就不用第三方库；包级变量与 `init` 不做重活。需要模型能力时调用 Agent 的 `llm.generate`（`sdk/llm`），不内嵌模型厂商 SDK。
- **副作用不自动重试。** 超时与取消不代表对端没执行。普通请求期限是 30s。
- **跨插件只走 Service 与 Event Bus。** 不读写其他插件的文件或数据库。Event Bus 是非持久广播，需要回执时用 Service Call。
- **JSON 泛型的类型参数**（`sdk.JSON`、`sdk.Subscribe`、`sdk.CallJSON`、`sdk.Publish`）必须是包级具名非泛型类型；改动后重新运行 `alonabot plugin generate`。

## 配置与存储

- 配置写在 `plugin.yaml` 同目录的 `config_schema.json`，从 `OnStart` 起用 `p.Config().String("key")` 等方法读取；在 `OnConfigLoad` 中或更早调用会 panic。声明格式、迁移与取值方法见 [references/config-and-storage.md](references/config-and-storage.md)。
- 业务数据放受管 SQLite：没有特别需要时用 `sqlitedb.Open(ctx)`（比 GORM 少约 1.5 MiB 常驻内存），需要 ORM 时用 `gormdb.Open(ctx)`。迁移实现 `Migrate` 或 `Migrations` 二者之一，已发布的迁移 ID 不改含义。

## 生成、检查与本地运行

```sh
alonabot plugin generate --dir ./myplugin   # 生成 alonabot_gen.go，内容不变时不改文件
alonabot plugin check --dir ./myplugin --goos linux --goarch amd64
go build -o bin/myplugin ./myplugin
alonabot run --data-dir ./data --plugin-id myplugin --entry bin/myplugin          # TCP loopback
alonabot run --data-dir ./data --plugin-id myplugin --entry bin/myplugin --stdio  # stdio
```

没有安装好的 `alonabot` 时，在 AlonaBot 源码目录用 `go build -o bin/alonabot ./cmd/alonabot` 构建。诊断以 `文件:行:列: 说明` 输出，退出码 1。会话失败、握手或帧错误按 [alonabot-wire-protocol](../alonabot-wire-protocol/SKILL.md) 排查。

## 完成前检查

- `go vet ./...` 与 `go build ./...` 通过，`alonabot plugin check` 不报过期或诊断。
- 每条注册链以 `Handle` 结束，权限按需显式设置（命令缺省要求 `user`）。
- 没有 stdout 输出、secret 明文、自动重试或未声明的用户可见中文/英文硬编码文案。
- 告诉用户实际运行了哪些命令；`alonabot run` 没有真实平台账号时，不把它说成真实平台验证。
