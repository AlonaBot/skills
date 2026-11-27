---
name: alonabot-plugin
description: 使用 AlonaBot v0.4.0 Go SDK 编写或修改功能插件、Agent 扩展时使用，涵盖命令、消息、Service、任务、配置、存储、测试与打包。平台接入使用 alonabot-adapter。
compatibility: AlonaBot 0.4.0；Go 1.27.1+；可读取 AlonaBot 源码或 Go module 缓存。
metadata:
  alonabot-version: "0.4.0"
  updated: "2026-11-27"
---

# 开发 AlonaBot 插件

> AlonaBot 0.4.0 · 最后更新 2026-11-27

先读取工程 `go.mod` 的 SDK 版本与 replace 路径，再按需求查看 SDK 源码、[插件规范](https://alonabot.github.io/docs/develop/rules)和 `examples/plugins/`。本技能按 Core 的 `v0.4.0` tag 核对；实际签名以工程使用的 SDK 为准。

## 创建与运行

```fish
alonabot init ./alonabot-plugin-hello
alonabot check ./alonabot-plugin-hello
alonabot run ./alonabot-plugin-hello --data-dir ./data
```

在终端输入 `/hello` 检查回复。带开发目录时默认热重载；`--reload=false` 关闭，`--stdio` 切换独立进程传输。独立 module 按 CLI 提示选择可访问的 SDK 版本；源码开发版可使用本地 replace。

包级入口导出 `New() *sdk.Plugin`，每次调用创建独立状态。在 `sdk.NewPlugin()` 返回的对象上注册行为：

```go
package main

import (
    "log/slog"
    "os"

    "github.com/AlonaBot/AlonaBot/sdk"
)

// New 创建问候插件并登记命令。
func New() *sdk.Plugin {
    p := sdk.NewPlugin()
    p.Command("hello").Menu().Handle(func(c *sdk.Context) error {
        return c.Reply(c.T("hello"))
    })
    return p
}

func main() {
    if err := sdk.Run(New()); err != nil {
        slog.Error("hello plugin stopped", "error", err)
        os.Exit(1)
    }
}
```

配套 `i18n/en-US.json`：`{"messages":{"hello":"Hello AlonaBot"}}`。`plugin.yaml` 声明身份，配置放 `plugin_config_schema.json`。构建工具将声明与翻译经 overlay 编入程序；随包 `assets/` 用 `sdk.PackageDir` 读取。直接使用 `alonabot build/check/run/pack`；工程无需生成文件或生成命令。

## 按任务读取

- 注册、参数绑定、消息、Service、Bus、任务、Agent 扩展：读 [SDK 用法](references/sdk-api.md)。
- 配置、secret、配置文件覆盖、KV、SQLite、插件文件：读 [配置与存储](references/config-and-storage.md)。

## 生命周期与调用

- 用 `p.OnConfigLoad(fn)` 登记纯配置变换；`p.OnStart(fn)` 打开资源，`p.OnReady(fn)` 完成激活前准备，`p.OnStop(fn)` 关闭资源并结束后台 goroutine。生命周期回调响应 ctx；业务调用从 ACTIVATED 后开始。
- `New()` 可被重复调用；状态放在本次创建的对象或闭包中。用 `p.Sub(id)` 划分一层逻辑模块，生命周期和迁移登记在根插件上。
- Handler 的 `c` 实现 `context.Context`，直接传给 SDK 分类包以保留期限、取消与父调用。后台工作使用所属生命周期的 context。
- 普通请求期限 30s。较长交互使用 `.Continue().Handle(...)` 或 `c.Continue(fn)`；在延续中用 `c.Wait(timeout)` 等待同一用户、会话和话题的下一条消息。延续随插件停止结束。
- 发送结果与错误可能同时存在，保留已确认消息句柄。有副作用的业务调用由调用者处理结果，避免自动重试导致重复执行。
- stdout 用于 stdio 协议，日志用 `log/slog` 或 `c.Logger()`；日志与错误用英文，用户文案用 i18n。secret 留在配置中，日志与业务数据省略凭证明文。

## 验证与打包

```fish
alonabot check .
go test ./...
alonabot test .
alonabot pack . --target linux/amd64 --target linux/arm64
```

`go test` 中用 `plugintest.Start(t, plugintest.Plugin(New()))` 启动真实 Core 与测试 Adapter，以 `h.Text(...)` 注入消息并用 `RequireReply` 检查行为，参考 `examples/plugins/echo/main_test.go`。`alonabot test` 验证离线握手、准备、注册、激活与停止；真实平台联调另需账号与目标会话。

`check/build/test/pack` 使用位置目录；`build/check` 的平台参数为 `--goos`、`--goarch`，`pack` 使用可重复的 `--target os/arch`。归档默认输出到工程 `bin/`。交付时列出改动与实际验证结果。
