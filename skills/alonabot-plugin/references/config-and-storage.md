# 配置与存储

> AlonaBot 0.4.0 · 最后更新 2026-11-27

## 配置

插件与 Agent 使用 `plugin_config_schema.json`，Adapter 使用 `adapter_config_schema.json`，与 `plugin.yaml` 同目录：

```json
{
  "greeting": {"type": "string", "description": "Greeting", "default": "Hello", "profile": true},
  "enabled": {"type": "bool", "default": true},
  "token": {"type": "string", "description": "Access token", "secret": true}
}
```

配置按字段声明，无版本号与迁移列表。需要改名搬移旧值时，用 `p.OnConfigLoad(func(ctx context.Context, raw json.RawMessage) (json.RawMessage, error))` 做可重复的纯变换，保留未知键。SDK 随后补值、无损转换和校正，required 仍缺失时准备失败；普通 restart 要求完整结果与原快照一致。

从 OnStart 开始读取 `p.Config()`；Handler 用 `c.Config()` 取得本次调用冻结的会话配置视图。`p.Sub(id).Config()` 读取 Schema 中同名 object 的基础配置。getter 与声明类型一致：

| 类型 | 方法 |
| --- | --- |
| string/text | `String(key)` |
| int/float/bool | `Int(key)`（int64）、`Float(key)`（float64）、`Bool(key)` |
| 无 items 的 list | `Strings(key)` |
| object / 对象 list | `Object(key)` / `Objects(key)` |
| dict | `Dict(key)`，值为 `json.RawMessage` |
| template/template_list | `Template(key)` / `TemplateList(key)`，`Key()` 取得选中模板 |

`profile: true` 允许会话配置文件覆盖字段。优先标记 Handler 使用的叶字段；启动连接、token 与后台缓存取基础配置。复合覆盖字段的整棵子树须无 secret。读取未声明键或类型不匹配的方法会 panic。

secret 仅用于 string/text，省略 default 与 options。Config 返回明文，空串表示未设置；业务数据、日志与错误省略凭证。配置翻译放 `i18n/<locale>.json` 的 `config`，用户回复放 `messages`，至少提供 en-US；`c.T(key, name, value, ...)` 取回复文案。命令英文说明写 `.Description(...)`，其他语言通过顶层 `commands` 按完整命令路径覆盖。

运行时选项源、模板、配置示例图片按需读取[配置教程](https://alonabot.github.io/docs/tutorial/config)。修改后运行 `alonabot check`，构建工具自动嵌入资源。

## KV 与 SQLite

简单状态使用 `sdk/kv`：

```go
err := kv.Set(c, "subscription/topic", []byte(topic))
value, ok, err := kv.Get(c, "subscription/topic")
```

`Get` 的 ok 表示键存在；`Delete` 删除，`List` 按前缀分页。KV 存入受管 SQLite，与业务数据一起按代次复制和回退。

需要 SQL 时使用 `sqlitedb.Open(ctx)`，需要 ORM 时使用 `gormdb.Open(ctx)`，两者共享受管连接。迁移在 New 中二选一登记：`sqlitedb.Migrate(p, fn)` 或 `sqlitedb.Migrations(p, list)`；GORM 使用对应包入口。版本化 Migration 的 ID 非零、唯一且递增，Up 使用传入事务，已发布 ID 保持原含义。

reload/update 使用独立候选数据库，准备失败保留旧数据；同代次 RESUME 复用数据库并跳过迁移回调。框架外数据库由插件管理，准备期间写外部数据时声明 `prepare_effects: external`。

## 插件文件

在就绪的 Handler 中通过 `sdk.PluginDataDir(c)` 取得插件数据目录，跨 restart 保留；更新回退保持文件现状。需要随代次回退的数据放 KV 或受管 SQLite。OnStart、OnReady 或停止后调用 PluginDataDir 会 panic，文件工作放在就绪之后执行。
