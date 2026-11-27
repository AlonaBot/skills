---
name: alonabot-adapter
description: 为 AlonaBot v0.4.0 接入聊天平台、修改或排查 Adapter 时使用，涵盖实例、事件与游标、媒体、发送编辑撤回、平台动作和账号配置页面。
compatibility: AlonaBot 0.4.0；Go 1.27.1+；可读取 AlonaBot 源码或 Go module 缓存。
metadata:
  alonabot-version: "0.4.0"
  updated: "2026-11-27"
---

# 开发 AlonaBot Adapter

> AlonaBot 0.4.0 · 最后更新 2026-11-27

先读 `sdk/adapter/`、`sdk/message/` 与 [Adapter 开发指南](https://alonabot.github.io/docs/develop/adapter)。官方参考实现位于独立仓库 `AlonaBot/alonabot-adapter-telegram`、`AlonaBot/alonabot-adapter-onebot11`；最小本地实现见 Core 的 `examples/devbot/adapter/`。

## 工程与入口

用 `alonabot init ./alonabot-adapter-example --role adapter --adapter-id example` 创建工程。`plugin.yaml` 声明 `role: adapter` 与平台类型 `adapter_id`；实例配置放 `adapter_config_schema.json`。市场 ID 使用 `alonabot-adapter-` 前缀、单连字符分词。

入口包导出 `New()` 返回实现 `adapter.Adapter` 的具体类型或接口，独立进程 `main` 调用 `sdk.RunAdapter(New())`。同一包的账号实例由一个进程承载。HTTP 路由在 `adapter.Descriptor.HTTP` 中声明。

| 接口 | v0.4.0 契约 |
| --- | --- |
| `Adapter.Descriptor() *adapter.Descriptor` | 平台能力、扩展、权限、事件、动作与 HTTP 路由 |
| `Adapter.Start(ctx, adapter.StartRequest, adapter.Core) (adapter.Instance, error)` | 解析配置、验证账号、启动接收后返回 |
| `Instance.Account() adapter.Account`、`Capabilities() []message.Capability` | 已验证账号与本实例能力 |
| `Instance.Send(ctx, to string, chain message.Chain) (*message.SendResult, *errcode.Error)` | 目标是会话字符串；部分成功保留回执和错误 |
| `Instance.Stop(ctx) error` | 返回前停止接收，结束实例 goroutine |
| `adapter.MediaSource` | 按平台 fileID 分块读取入站媒体 |
| `adapter.Editor`、`adapter.Deleter` | 编辑、撤回；按能力实现可选接口 |
| `adapter.NodesReader` | 读取合并转发节点 |
| `adapter.ActionExecutor` | 执行类型声明对应的平台动作 |
| `adapter.CommandSynchronizer`、`adapter.CommandMenuController` | 命令菜单同步与按 scope 管理 |

## 事件与媒体

- 事件使用 `message.Event`。`Session` 由 `message.SID(adapterID, chatID, threadID)` 构造，实例身份与当前 Core 绑定一致；事件 `ID` 交给 Core 分配。Actor 提供平台身份和权限，由 Core 计算有效权限。
- 从 `StartRequest.Cursor` 恢复。每条事件携带处理后的游标，等待 `Core.Deliver` 返回后处理下一条；`accepted=false` 表示队列满且游标已保存，继续推进。没有事件时用 `SaveCursor`。
- `StartRequest.ConfigJSON` 含凭证，解析错误只报告字段与原因；日志、错误和事件省略凭证明文。
- 实现 `ReadPlatformFile` 提供入站媒体，单次读取 1 B～1 MiB。出站用 `Core.ReadMedia`；需要平台可抓取 URL 时用 `Core.MediaURL` 取得临时地址。
- 不可恢复故障调用 `Core.ReportFailure`。连接恢复按平台协议处理；已提交的发送保留原结果，避免重连后重复发送。

## 声明与发送

- 整条 Chain 先校验并生成有序物理操作，再调用平台。无法表达的组合在副作用发生前返回 `errcode.Unsupported`，保留片段次序、引用与每个已确认的平台消息 ID。
- 撤回按输入顺序保留各项结果，遇平台失败或结果未知后停止，余项标为 `PartNotAttempted`。编辑返回完整回执及错误。
- 扩展片段用 `adapter.Extension[T]`，扩展事件用 `adapter.Event[T]`，动作用 `adapter.Action[P, R]` 声明名称、版本和 scope；从 Go 类型推导 Schema。插件侧共享载荷类型，使用 `sdk.AdapterEvent`、`sdk/action`。
- 平台权限和扩展事件完整名带 `<adapter_id>.` 前缀，后缀使用小写字母、数字与下划线。权限隐含关系可指向同平台权限或 `chat_admin`，保持无环。
- 给插件复用的权限、事件、动作放在轻量声明包，避免链接 Adapter 运行时。Descriptor 的能力逐项对应已实现行为。
- 账号登录页面在 Descriptor 中登记 GET、零值 Auth 的路由，`Page.Purpose` 为 `adapter.PageInstanceConfig`。使用 WebUI `kit.js` 的 `getConfigContext`、`completeConfig({patch})`，用户保存后生效；完整桥协议见插件指南「Adapter 账号配置页面」。

## 开发验证

```fish
alonabot check ./alonabot-adapter-example
alonabot test ./alonabot-adapter-example
alonabot run ./myplugin ./alonabot-adapter-example --data-dir ./data
```

`test` 验证 Adapter 进程准备，保持零平台实例。`run` 的账号实例从数据目录的 `core_config.json` 中读取。平台测试用本地模拟服务和假的 `adapter.Core`，检查账号验证、游标续接、队列满、媒体、部分发送成功、Stop 后停止交付。

运行中可用 `alonabot adapter instance list`、`alonabot adapter actions <instance>` 发现实例及操作，再按返回的 Schema 联调。`sdk/debug` 和 `plugintest.Connect` 可连接已认证的 Core。调用平台动作会产生实际副作用，沿用用户授权范围。交付中区分模拟服务与真实账号的验证结果。
