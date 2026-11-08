---
name: alonabot-wire-protocol
description: 排查 AlonaBot 插件或 Adapter 与 Core 之间的会话问题时使用，例如握手被拒、SESSION_ERROR、PROTOCOL_ERROR、帧超过 4 MiB、准备或激活卡住、调用超时与取消、RESPONSE 同时带结果和错误、stdout 污染 stdio 传输，或需要理解 Envelope 字段、错误码与生命周期帧。Use when debugging the AlonaBot RPC wire format. 只是写业务功能时改用 alonabot-plugin 或 alonabot-adapter。
compatibility: AlonaBot 0.1-beta（传输协议 1.0）；需要能读取 github.com/AlonaBot/AlonaBot 源码。
metadata:
  alonabot-version: "0.1-beta"
  updated: "2026-11-08"
---

# 排查 AlonaBot Wire 协议

> AlonaBot 0.1-beta · 最后更新 2026-11-08

产品版本 0.1 与传输协议版本 1.0 分开。权威是 AlonaBot `spec/WIRE-FORMAT.md`（字段号、帧方向、错误码）与 `spec/CONTRACTS-v0.1.md` §1、§4、§12；`.proto` 在 `contracts/proto/alonabot/v1/`，Go 类型在 `contracts/pb/alonabotv1`。组件作者通常不直接构造帧，SDK 负责编解码；本技能用来把症状对应到阶段和原因。

## 帧与会话

- 每帧是 4 字节大端长度加 Protobuf Envelope，长度 1～4,194,304 字节；0、超限或无法解码的帧让会话失败。TCP loopback 与 stdio 编码相同。
- 生命周期：HELLO → HELLO_ACK（含 Bootstrap）→ REGISTER → PREPARED → ACTIVATE → ACTIVATED → 业务 REQUEST/RESPONSE/CANCEL/EVENT → STOPPING。HEARTBEAT 只做诊断；SESSION_ERROR 表示阶段失败，发送后关闭会话。
- OnStart 与 OnReady 期间不能发普通业务 Send、Service 或 Publish；SDK 写完 ACTIVATED 后才放开业务。

## 症状对照

| 症状 | 先查 |
| --- | --- |
| stdio 下立即 PROTOCOL_ERROR 或帧长度异常 | 有代码往 stdout 打印（`fmt.Println`、第三方库日志）；改用 `log/slog`，它写 stderr |
| HELLO 后连接被关闭 | 插件 ID、启动 secret、包摘要或协议主版本不符；未知的 required capability（例如有 HTTP 路由时的 `http_listener`） |
| CONFIG 阶段 SESSION_ERROR | `config_schema.json` 声明、`$migrations`、`OnConfigLoad` 返回值或缺失必填字段；普通 restart 时 OnConfigLoad 改了配置 |
| MIGRATION 阶段 SESSION_ERROR | 迁移失败、超出迁移阶段预算（调优参数 `PHASE_MIGRATION`，缺省 15m），或已发布迁移 ID 改了含义 |
| REGISTER 被拒或 RESOURCE_EXHAUSTED | 注册链缺 `Handle`、路由重复、注册描述整体超过一帧 4 MiB、Schema hash 与生成文件不一致（重新 `alonabot plugin generate`） |
| 业务调用 PLUGIN_NOT_READY | 在 OnStart/OnReady 中发起了业务调用，或目标插件尚未激活 |
| DEADLINE_EXCEEDED、CANCELLED | 普通请求期限 30s，子请求不能延长父期限；取消不代表对端没执行，不要自动重试有副作用的调用 |
| CALL_CYCLE | Service 调用链回到了自己 |
| RESOURCE_EXHAUSTED（运行中） | 每 Session 在途请求数或载荷预算耗尽，Event Bus 订阅回调阻塞会占住接收预算 |
| NO_DEFAULT_TARGET | 后台、订阅回调或 schedule 中 Send 没写 To |

错误码固定为 INVALID_ARGUMENT=1 到 NEEDS_MANUAL_MIGRATION=17，未知码对调用者映射为 INTERNAL；完整表与 Envelope 字段见 AlonaBot `spec/WIRE-FORMAT.md` §1–§2。

## RESPONSE 的结果与错误

RESPONSE 的 payload 与 ErrorInfo 不是 oneof：部分发送成功时同时返回 SendResult 和错误。处理时先保留已确认的结果，再看错误；不要因为 error 非空丢掉消息 ID。`safe_message` 不含 secret、堆栈或本地绝对路径，只用英文。

## 排查步骤

1. 用 `alonabot run --log-level debug` 复现，记下失败阶段（failed_phase、failed_kind）与错误码。
2. 按上表定位到阶段，再读 WIRE-FORMAT 与 CONTRACTS 对应小节确认规则，不凭猜测改超时或加重试。
3. 调优参数只通过 `--args KEY=VALUE` 临时覆盖（`alonabot tune list` 查看），改动缺省值需要契约依据。
4. 向用户报告时给出阶段、错误码、触发原因和已运行的命令；没复现成功就说没复现。
