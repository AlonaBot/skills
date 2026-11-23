# SDK 用法

> AlonaBot 0.3.0 · 最后更新 2026-11-23

## 注册与参数

| 需求 | 写法 |
| --- | --- |
| 命令与分组 | `p.Command("hello").Menu().Description("Say hello").Handle(h)`、`p.Group("note").Command("add")` |
| 文本与原始消息 | `p.Regex(expr)`、`p.Keyword(patterns...)`、`p.StartsWith(...)`、`p.EndsWith(...)`、`p.FullMatch(...)`、`p.Raw()` |
| 事件 | `p.On(sdk.MessageCreated).Handle(h)`，可接收类型化 payload |
| Hook | `p.PreEvent(sdk.MessageCreated)`、`p.BeforeSend()`、`p.AfterSend()` |
| HTTP | `p.Route(http.MethodGet, "/status").HandleFunc(h)`；私有路由要求 Core 管理会话 |
| 静态任务 | `p.Every("tick", time.Minute).Handle(fn)`、`p.Cron("daily", expr).Timezone(tz).Handle(fn)` |

每个 builder 恰好以 `Handle` 或其等价入口结束一次。文本、事件和 Hook 默认生成 route ID；同类重复或模式会变化时用 `.ID("stable-name")` 固定身份。`Sub` ID 保留逻辑归属，命令路径通过 Group 显式增加前缀。

命令 Handler 首参为 `*sdk.Context`，返回 `error` 或 `(message.Content, error)`。位置参数支持 `string`、`int64`、`int`、`float64`、`bool`、`sdk.UserID`、`*message.ImageSegment`、`message.Chain`、`time.Duration`，具体绑定以 `sdk/arguments.go` 为准。源码参数名由构建工具记录，可用 `.Args(...)` 显式覆盖；`Optional`、`RawTail` 在 builder 上设置。

多个可选参数可改用一个结构体：字段顺序决定位置，`arg:"name"` 指定名字，指针表示可选，`default:"…"` 指定默认值，末字段 `arg:",raw"` 接收尾部。结构体方式独立使用。

`c` 提供 `Text()`、`Event()`、`Match()`、`Has(p)`、`Config()`、`T(...)`、`Reply(...)`、`Send(...)`。命令与文本匹配默认要求 `sdk.User`，可用 `.Permission(sdk.Anyone)` 等显式设置；优先取路由、最近组、外层组的权限。事件默认空权限集合。

## 消息与句柄

- 内容用 `sdk/message`：`message.Text`、`ImageURL`、`Mention`、`Quote` 或 `message.Chain`。自动回复返回 `(message.Content, error)`；nil 和空 Chain 表示无回复。
- `c.Reply(text)` 返回 error；需要回执用 `c.Send(content)`。跨会话用 `c.SendTo(sid, content)`，后台用 `sdk.Send(ctx, sid, content)`；`sdk.Via(instanceID)` 指定实例。
- 会话字符串格式 `<adapter_id>:<chat_id>[#<thread_id>]`，用 `message.SID` 构造并转义，`ParseSID` 拆解。用户身份用 `message.UID`。
- 消息句柄格式 `<sid>/<platform_message_id>[@<instance>]`，用回执 `Ref()`、`Refs()` 或 `c.Event().Ref()` 取得；手动构造用 `message.Ref`。编辑用 `c.Edit(ref, content)`，引用用 `message.Quote(ref)`，撤回签名查 `sdk/delete.go`。
- 错误通过 `errcode.Of(err)` 检查；发送或编辑返回 error 时保留非空回执。入站媒体用 `media.Open(ctx, source)`，其余操作按需查看 `sdk/media/`。

## Service 与 Bus

类型声明与服务目标绑定，注册和调用共用声明：

```go
type Query struct { Name string `json:"name"` }
type Result struct { Text string `json:"text"` }

var lookup = service.JSON[Query, Result]("/lookup")

func register(p *sdk.Plugin) {
    service.Serve(p, lookup).Access(service.AccessPublic).
        Handle(func(c *sdk.Context, q Query) (Result, error) {
            return Result{Text: q.Name}, nil
        })
}
```

`/lookup` 在注册时绑定本插件，跨插件调用用完整的 `plugin-id/lookup` 声明后 `.Call(c, input)`。默认 AccessPrivate，另有 AccessAllowlist 和 AccessPublic。Agent 工具在 Handle 前用 `.Tool(description, tool.DefaultAllow/DefaultConfirm)`，需要标注副作用时紧接 `.Effect(...)`；枚举见 `sdk/tool`。

JSON 类型使用包级具名非泛型类型，Schema 由构建工具推导；版本通过 `SchemaVersion() uint32` 声明，默认 1。自有 Protobuf 服务用 `service.Proto`。

- 流式服务：`service.StreamJSON[In, Delta, Out]`、`service.ServeStream`，调用 `.Call(ctx, in, onDelta)`，Handler 接收 emit 回调。增量与最终结果分别处理。
- Bus：`bus.Define[T]("plugin-id/event")` 声明，`bus.Subscribe(p, event).Handle(fn)` 订阅，`event.Publish(c, payload)` 发布。Bus 为非持久广播；回调串行执行，发送时显式给出会话。

## 任务与 Agent 扩展

- 动态任务：`task.Define[Args](name)`、`task.On(p, typ).Handle(fn)`，调用 `typ.At/Every/Cron(ctx, ..., args, opts...)`。`task.Key` 指定可替换实例，`typ.Cancel` 取消，`typ.List` 查询。静态任务可用 `task.Pause/Resume/Status` 管理。
- 模型生成：`llm.Generate`、`llm.GenerateStream` 调用 `@agent` 的 `llm.generate`，需要管理员授予插件访问权。
- Agent 运行：`agent.Run`、`agent.RunStream`；生命周期扩展使用 `sdk/agent/hook`，持久 goal 使用 `sdk/agent/goal`。选用前读 `docs/agent-api.zh-CN.md` 中对应参数和权限。
- 会话数据用 `conversation.Append/List/Clear`；平台动作用 `sdk/action`；按钮用 `sdk/button`。从对应分类包读取实际签名。

## Manifest

作者填写 `id`、`name`、SemVer `version`、`source`，按需填 `description`、`author`、`role`、`icon`、依赖与 `prepare_effects`。构建字段由 CLI 补齐。

市场 ID 按角色使用 `alonabot-plugin-`、`alonabot-adapter-`、`alonabot-agent-`、`alonabot-webui-` 前缀，后缀以单连字符分词，最长 64 字符。本地 ID 为 `^[a-z][a-z0-9_-]{0,63}$`。依赖引用实际组件 ID。

可按重启语义恢复且没有独立后台活动的 plugin/agent 可声明 `sleep_capable: true`；需持续运行的组件保持常驻。发布归档用 `alonabot pack` 生成。
