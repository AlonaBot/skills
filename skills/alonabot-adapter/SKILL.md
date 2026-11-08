---
name: alonabot-adapter
description: 为 AlonaBot 编写、修改或排查平台 Adapter 时使用，包括用 sdk.RunAdapter 与 sdk/adapter 实现 Adapter、Instance 与 MediaSource 接口，交付事件与游标，声明能力、平台权限、扩展片段与扩展事件，编写 instance_schema.json，以及用本地模拟服务测试。Use when connecting a new chat platform to AlonaBot. 编写普通功能插件时改用 alonabot-plugin。
compatibility: AlonaBot 0.1-beta；Go 1.27+；需要能读取 github.com/AlonaBot/AlonaBot 源码。
metadata:
  alonabot-version: "0.1-beta"
  updated: "2026-11-08"
---

# 编写 AlonaBot Adapter

> AlonaBot 0.1-beta · 最后更新 2026-11-08

Adapter 是 `role: adapter` 的组件：独立 Native 进程，一个进程承载该包的全部账号实例，复用插件的握手、Session、准备/激活/停止状态机、配置与 Secret。权威条款是 AlonaBot `spec/CONTRACTS-v0.1.md` §8 与 `spec/MESSAGE-MODEL.md`；参考实现是 `adapters/telegram/`，本地 Bot API 模拟服务是 `adapters/telegram/telegramtest/`。与仓库冲突时以仓库为准。

## 先确认现状

- 读取 `sdk/adapter/adapter.go` 的接口与 `adapters/telegram/` 的写法；不凭记忆补字段。
- 0.1-beta 中 Telegram 只实现了接收与入站媒体，发送、出站媒体、按钮由后继版本实现；`alonabot run` 目前只运行插件，没有运行 Adapter 的开发命令。验证靠 Adapter 自己的测试加本地模拟服务。

## 入口与接口

```go
func main() {
	if err := sdk.RunAdapter(myplatform.New()); err != nil {
		slog.Error("myplatform adapter stopped with an error", "error", err)
		os.Exit(1)
	}
}
```

| 接口（`sdk/adapter`） | 职责 |
| --- | --- |
| `Adapter.Descriptor() *pb.AdapterDescriptor` | `adapter_id`、能力、扩展片段、平台权限与扩展事件声明 |
| `Adapter.Start(ctx, StartRequest, Core) (Instance, error)` | 校验实例配置与凭证、用平台 API 验证账号、启动接收后返回 |
| `Instance.Account()`、`Capabilities()` | 已验证的账号身份与本实例能力 |
| `Instance.Send(ctx, target, chain) (*pb.SendResult, *pb.ErrorInfo)` | 发送；部分成功时两者同时返回 |
| `Instance.Stop(ctx)` | 返回前停止接收 |
| `MediaSource.ReadPlatformFile(...)`（可选） | 供 Core 按需取入站媒体，单次 1 B～1 MiB；不实现时 SDK 不声明 `media.read` |
| `Core.Deliver`、`SaveCursor`、`ReportFailure`、`ReadMedia` | 交付事件与游标、保存空闲游标、报告致命故障、读取出站媒体 |

## 必须遵守的规则

1. **配置边界。** `StartRequest.ConfigJSON` 含凭证，只在 Start 中解析；错误只报字段名，不回显值。凭证不进日志、错误、事件与普通响应。实例配置的 schema 写在包内 `instance_schema.json`。
2. **事件归属。** 交付事件的 Target 与 adapter_id、adapter_instance_id、account_id 必须等于本实例；event_id 由 Core 生成，Adapter 不填。Actor 只给平台身份与平台权限，权限集合由 Core 计算。
3. **逐条交付游标。** 每条事件附交付后的不透明游标，等 `Deliver` 返回后再处理下一条；`accepted=false` 表示队列满已丢弃，游标已持久化，不重放。没有事件时的游标推进用 `SaveCursor`。启动时从 `StartRequest.Cursor` 续接。
4. **入站媒体。** 平台文件标识留在 Adapter，Core 以 token 映射，首次读取时经 `ReadPlatformFile` 取件。
5. **发送计划先于副作用。** 整条 Chain 先验证并生成有序物理操作，无法表达的组合在首个平台调用前返回 UNSUPPORTED；不静默丢弃或重排 Reference、按钮、相册与文本。保留每个已确认的平台消息 ID。
6. **声明命名。** 平台权限与扩展事件名只用小写字母、数字、下划线，完整名带 `<adapter_id>.` 前缀；权限可隐含同 Adapter 已声明的权限或 `chat_admin`，不能隐含 `user`、`admin`，不能成环。扩展片段与扩展事件带版本和 JSON Schema。
7. **给插件的常量包。** 权限与扩展事件放在只含常量与数据类型的包（如 `adapters/<id>/perm`、`adapters/<id>/event`），只 import `sdk` 根包，插件不链接 Adapter 运行时。
8. **故障与重连。** 不可恢复的故障用 `ReportFailure` 报告（之后 `Deliver` 返回错误）；网络错误有界退避重连，重连不重放已提交的 Send；代理失败不改为直连。
9. **manifest。** `plugin.yaml` 写 `role: adapter` 与 `adapter_id`（小写平台类型），不得声明插件路由、HTTP、数据库或迁移；市场 ID 用 `alonabot_adapter_` 前缀。同一 `adapter_id` 同时只能安装一个包。
10. **stdout 是协议通道**，日志用 `log/slog`，内容用英文。

## 测试

- 为平台写本地模拟服务（参照 `telegramtest`），在 `go test` 中驱动 `Start`、事件交付、游标、媒体读取与 Stop，用假的 `adapter.Core` 记录交付。
- 真实平台验证需要用户提供测试账号；没有时如实写「未运行」，不把模拟服务结果说成真实平台验证。

## 完成前检查

- `go vet ./...`、`go test ./...` 通过，测试覆盖账号验证失败、游标续接、队列满、Stop 后交付失败。
- 凭证不出现在任何日志、错误字符串与测试快照中。
- Descriptor 声明的能力与实际实现一致；未实现的发送组合返回 UNSUPPORTED。
