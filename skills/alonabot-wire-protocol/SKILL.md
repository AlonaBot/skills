---
name: alonabot-wire-protocol
description: 排查 AlonaBot v0.4.0 组件与 Core 的握手、SESSION_ERROR、PROTOCOL_ERROR、准备或激活失败、超时取消、部分成功与 stdio 污染时使用。
compatibility: AlonaBot 0.4.0；传输协议 1.0；可读取 AlonaBot 源码。
metadata:
  alonabot-version: "0.4.0"
  updated: "2026-11-27"
---

# 排查 AlonaBot Wire 协议

> AlonaBot 0.4.0 · 最后更新 2026-11-27

从 Core 日志的 `failed_phase`、`failed_kind`、错误码与 trace 定位阶段，再查 `contracts/proto/alonabot/v1/`、`contracts/validate/`、`internal/wire/` 与 SDK 对应实现。SDK 公共消息类型在 `sdk/message`，可检查错误在 `sdk/errcode`。

## 帧与就绪

- 传输协议为 1.0。每帧由 4 字节大端长度与 Protobuf Envelope 组成，Envelope 长度为 1～4,194,304 字节。TCP loopback 与 stdio 使用相同编码。
- 生命周期：HELLO → HELLO_ACK（Bootstrap）→ REGISTER → PREPARED → ACTIVATE → ACTIVATED → 业务 → STOPPING。失败时 SESSION_ERROR 报告阶段并关闭会话。
- `Bootstrap.package_dir` 指向独立组件的包目录，开发和测试时为工程目录，直接使用库模式时为空。
- HELLO_ACK 起使用 Core 分配的 Session ID；REQUEST/RESPONSE/CANCEL 保留原发起方向和 call_id。parent_call_id 只引用同 Session 中仍在途的 Core 请求；子调用继承父期限。
- OnStart、OnReady 完成后写出 ACTIVATED，SDK 才放行业务调用。HEARTBEAT 用于诊断。

## 按症状定位

| 症状 | 核对内容 |
| --- | --- |
| stdio 下首帧长度异常或 PROTOCOL_ERROR | stdout 是否混入打印或第三方日志；改用写 stderr 的 `log/slog` |
| HELLO 后断开 | 认证 ID、启动 secret、构建摘要、协议主版本与 required capabilities |
| CONFIG 失败 | `plugin_config_schema.json`、OnConfigLoad、required 字段；restart 的完整配置须与原快照一致 |
| MIGRATION 失败 | SQLite 迁移错误、迁移 ID 变化和阶段预算；同代次 RESUME 跳过数据库迁移回调 |
| REGISTER 失败 | builder 是否恰好 Handle 一次，route ID 是否重复，资源是否由 `alonabot build` 编入，注册描述是否超帧限 |
| PLUGIN_NOT_READY | 调用是否早于 ACTIVATED，或目标组件尚未就绪 |
| DEADLINE_EXCEEDED / CANCELLED | 原请求剩余期限、父调用、取消传播；保留已完成的业务结果 |
| `core.package.list` / `core.package.install` 被拒绝 | 核对 `parent_call_id` 仍在途及原始消息 Actor 的 Core 管理员身份；安装失败时检查响应中已有的 `affected` |
| CALL_CYCLE | Service 调用链是否成环，服务是否按实际需要声明 Reentrant |
| 运行中 RESOURCE_EXHAUSTED | Session 在途请求与载荷预算、阻塞的 Bus 回调 |
| NO_DEFAULT_TARGET | 后台、Bus 或任务中显式提供会话字符串 |
| HTTP 返回 401 | 管理会话或凭证；公开回调由插件校验其声明的签名或 state |

Service 调用的 `InvocationContext.chat_type` 由 Core 从真实入站事件继承，和用户及会话一起传给下游。

错误码 1～17 的定义见 `contracts/proto/alonabot/v1/errors.proto`，业务代码通过 `errcode.Of(err)` 检查包装后的错误。RESPONSE 可同时包含 payload 和 ErrorInfo；先保留回执及已确认消息，再处理错误。

## 复现与修复

```fish
alonabot run ./myplugin --data-dir ./data --log-level debug
alonabot run ./myplugin --data-dir ./data --log-level debug --stdio
```

按故障涉及的传输选择复现。若日志指向资源缺失，用 `alonabot check ./myplugin` 查看静态诊断，再用 `alonabot build` 或 `run` 重建；扫描 manifest、Schema、i18n 与 Handler 参数由构建工具完成。

需要核对预算时运行 `alonabot tune list`，以 `--args KEY=VALUE` 临时覆盖；先依据失败阶段和当前值确认原因。普通请求期限 30s，业务副作用保留原调用结果，避免自动重试。对用户报告阶段、错误码、原因、修复与实际复现结果。
