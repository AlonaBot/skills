# AlonaBot SDK 0.1-beta 速查

> AlonaBot 0.1-beta · 最后更新 2026-11-08

摘自 `github.com/AlonaBot/AlonaBot/sdk` 的导出 API。签名以源码为准；这里与源码不一致时按源码写代码，并提醒用户更新本技能。

## 目录

- [注册入口](#注册入口)
- [Handler 与参数绑定](#handler-与参数绑定)
- [权限](#权限)
- [发送](#发送)
- [Service 与 Event Bus](#service-与-event-bus)
- [HTTP](#http)
- [媒体与 Agent 会话](#媒体与-agent-会话)
- [Manifest 与 ID](#manifest-与-id)

## 注册入口

| 根插件方法 | 注册器 | 常用链 |
| --- | --- | --- |
| `Commands(r *sdk.CommandRegistry)` | 命令与文本匹配 | `r.Group("demo")`、`r.Command("a", "b")`、`r.Regex(id, expr)`、`r.Keyword(id, ...)`、`r.StartsWith(id, ...)`、`r.EndsWith(id, ...)`、`r.FullMatch(id, ...)`、`r.Raw(id)` |
| `Events(r *sdk.EventRegistry)` | 事件路由 | `r.On(routeID, "message.created").Priority(n).Permission(p).Handle(h)` |
| `Services(r *sdk.ServiceRegistry)` | 类型化 Service | `sdk.JSON[In, Out](r, name).Access(...).Tool(...).Handle(h)` |
| `Subscriptions(r *sdk.SubscriptionRegistry)` | Event Bus 订阅 | `sdk.Subscribe[T](r, routeID, eventName).Handle(func(*sdk.Context, T) error)` |
| `HTTP(r *sdk.HTTPRegistry)` | HTTP 路由 | `r.Handle(method, path, handler, opts...)` |
| `Schedules(r *sdk.ScheduleRegistry)` | 定时任务 | 0.1-beta 尚不支持，注册后 `Run` 返回 UNSUPPORTED |

`CommandBuilder` 的选项：`Alias(path...)`、`Args(names...)`、`Optional(name, def...)`、`RawTail()`、`UsageKey(key)`、`Silent()`、`IgnoreCase()`、`Priority(p)`、`Permission(p)`，最后 `Handle(handler)`。`Group` 返回子注册器，也可以 `Permission`。

命令按最长完整路径与别名优先匹配；命令前缀由实例配置决定，不写在路径里。Handler 里用 `c.Match()` 取文本匹配的分组。

## Handler 与参数绑定

命令与文本匹配的 Handler 第一个参数必须是 `*sdk.Context`，返回 `error`（手动型）或 `(message.Chain, error)`（自动回复型），不能是变参。其后的参数按位置绑定，支持的类型：

| Go 类型 | 参数类型 |
| --- | --- |
| `string` | 字符串 |
| `int64` | 整数 |
| `float64` | 浮点数 |
| `bool` | 布尔 |
| `sdk.UserID` | 用户（mention 或 ID） |
| `*pb.ImageSegment` | 图片 |
| `*pb.MessageChain` | 消息 |

参数错误时 Core 回复 usage；`UsageKey` 指定 i18n 文案键，`Silent` 不回复。事件 Handler 签名是 `func(*sdk.Context) error`，用 `c.Event()` 读取 `*pb.Event`。

`Context` 常用方法：`Context()`、`Event()`、`Logger()`、`Match()`、`Has(p)`、`Caller()`、`Send(chain, opts...)`、`StopPropagation()`。

## 权限

`sdk.Anyone`、`sdk.User`、`sdk.Admin`、`sdk.ChatAdmin`、`sdk.PermissionName("adapter.perm")`，用 `p.Or(...)` 取并集。命令与文本匹配取最近一层显式设置（路由、最近的组、外层的组），都没有时要求 `user`；事件路由没有设置时为空集合。Handler 内用 `c.Has(p)` 按当前 Actor 判断。

## 发送

- `c.Send(chain)` 发到当前事件的原 Target，无需额外许可。
- `c.Send(chain, sdk.To(target))` 或后台 `sdk.Send(ctx, target, chain)` 发到其他目标，需要该插件的实例发信许可；无默认目标又没写 To 时返回 NO_DEFAULT_TARGET。
- 返回的 `*pb.SendResult` 与 error 可能同时存在（部分成功），不要因 error 非空丢掉已确认的消息 ID。
- 显式 Send 空 Chain 是参数错误；自动回复型返回空 Chain 表示不回复。
- `sdk/message` 目前提供 `message.Text(s)`；其他片段直接使用 `pb` 类型，以源码为准。

## Service 与 Event Bus

- `sdk.JSON[In, Out](r, "name")` 注册 JSON Service；`Access(sdk.AccessPrivate | sdk.AccessAllowlist | sdk.AccessPublic, allowlist...)` 控制调用方；`Tool(description, sdk.ToolAllow | sdk.ToolConfirm)` 把它交给 Agent 作为工具。
- 调用其他插件：`out, err := sdk.CallJSON[In, Out](ctx, "plugin_id/service", in)`；在 Handler 内调用会关联父请求。
- 输入不符合 Schema 时调用方收到 INVALID_ARGUMENT 与固定文本 `invalid service input`，Handler 不被调用。
- 发布：`sdk.Publish[T](ctx, eventName, payload)`，只能发布到本插件命名空间；成功只表示 Core 准入，不保证送达。
- 订阅回调串行执行，不要阻塞；回调里没有 Actor 与默认 Target，发消息要显式 To。
- 类型参数必须是包级具名非泛型类型；Schema 版本取类型的 `SchemaVersion() uint32`，没有时为 1。

## HTTP

`r.Handle(http.MethodGet, "/status", h)` 注册私有路由，`sdk.Public(pb.HttpAuthMode_HTTP_AUTH_MODE_CALLBACK_SIGNATURE)` 等声明公开回调（插件自己验证 state、签名或 nonce）。Core 经 `/plugins/{id}/...` 转发。0.1-beta 的私有路由尚未接入授权，一律 401；本地调试见 AlonaBot `docs/development.md`。

## 媒体与 Agent 会话

- `sdk.Upload(ctx, r, size, mime, sum)` 分块上传，返回 `*pb.MediaRef`。
- `sdk.ReadMedia(ctx, token, offset, length)` 读取入站媒体；`sdk.GrantMedia(ctx, token, granteePluginID, targets...)` 转授只读 token。
- Agent 会话：`sdk.AppendConversation`、`sdk.ListConversation`、`sdk.ClearConversation`。
- 模型能力：`sdk/llm` 以 `@agent` 为目标调用 `llm.generate`，需要管理员把插件加入 `llm_service_allowed_plugins`。

## Manifest 与 ID

- 所有 ID 最长 64 个 ASCII 字符。本地 ID：`^[a-z][a-z0-9_-]{0,63}$`，拒绝 Windows 保留设备名。
- 市场 ID：`alonabot_plugin_` 前缀，`^alonabot_plugin_[a-z][a-z0-9]*(?:_[a-z0-9]+)*$`；Adapter、Agent、WebUI 依次为 `alonabot_adapter_`、`alonabot_agent_`、`alonabot_webui_`。
- `plugin_dependencies`：plugin_id、可选 `min_version`（SemVer）与是否必需；`adapter_dependencies`：adapter_id、所需 extension 与版本、是否必需。
