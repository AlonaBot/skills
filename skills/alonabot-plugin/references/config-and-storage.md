# 配置、secret 与受管 SQLite

> AlonaBot 0.1-beta · 最后更新 2026-11-08

权威条款：AlonaBot `spec/CONTRACTS-v0.1.md` §5（配置）、§6（数据库）。

## config_schema.json

写在 `plugin.yaml` 同目录，按字段键声明类型与英文源文案：

```json
{
  "$version": 2,
  "$migrations": [{"to": 2, "rename": {"hello": "greeting"}}],
  "greeting": {"type": "string", "description": "Greeting", "default": "Hello"},
  "enabled": {"type": "bool", "default": true},
  "token": {"type": "string", "description": "Access token", "secret": true}
}
```

- `$version` 缺省为 1。SDK 从已保存的版本依次执行 `$migrations`，首装跳过；每步按 `rename` → `map` → `wrap` → `remove` 执行，路径不存在时跳过，rename 目标已存在时失败。
- 声明式迁移后调用根插件的 `OnConfigLoad(ctx, fromVersion, raw)`：保留未知键、响应 ctx、只做可重复的纯配置变换；`sdk.Base` 默认原样返回。普通 restart 与恢复时改变配置会返回 FAILED_PRECONDITION。
- 之后 SDK 只给缺失键补 `default` 或类型零值，再校验。显式 `false`、`0`、空串与空集合保持原样。
- 改动后运行 `alonabot plugin generate`，规范 JSON 嵌入 `alonabot_gen.go`。

## 读取配置

从 `OnStart` 起用 `p.Config()` 取得 `config.Values`，同一 Session 内不变，可并发读取：

| 声明类型 | 取值方法 |
| --- | --- |
| `string`、`text` | `String(key)` |
| `int`、`float`、`bool` | `Int(key)`（int64）、`Float(key)`（float64）、`Bool(key)` |
| 没有 `items` 的 `list` | `Strings(key)` |
| `object`、有 `items` 的 `list` | `Object(key)`、`Objects(key)` |
| `dict` | `Dict(key)`，返回 `map[string]json.RawMessage` |
| `template`、`template_list` | `Template(key)`、`TemplateList(key)`，`Key()` 取选中的模板键 |

读取未声明键或类型不符的方法会 panic。

## secret

只用于 `string` 与 `text`：声明 `"secret": true`，同时省略 `default` 与 `options`。`Config()` 取到明文；空串表示未设置。明文不进日志、错误与业务数据库。

## 翻译

`i18n/<locale>.json` 的 `config` 根节点按字段层级覆盖 `description`、`hint`、`labels`，子字段放在 `items`，模板放在 `templates`；用户可见回复放在 `messages`；`metadata` 翻译 manifest 的 `name` 与 `description`。

```json
{"config": {"greeting": {"description": "问候语"}}, "messages": {"hello": "你好"}}
```

## 受管 SQLite

- `sqlitedb.Open(ctx)` 返回 `*sql.DB`；`gormdb.Open(ctx)` 返回 `*gorm.DB`，两者共用同一个连接。不导入就不链接，`sqlitedb` 少约 1.5 MiB 常驻内存。
- 迁移二选一，实现在根插件上：
  - `Migrate(ctx, db *sql.DB) error`：自管版本，须可重复执行。
  - `Migrations() []sqlitedb.Migration`：`ID` 非零、唯一、严格递增，`Up` 必须使用传入的 `tx`；发布后的 ID 不能改成另一项操作。
- reload 与 update 在独立候选副本上准备，失败回到旧数据；框架外文件与外部数据库不在这个保证内，写外部数据时在 `plugin.yaml` 声明 `prepare_effects: external`。
- 大数据放磁盘，内存缓存必须有上限。Core 托管的 KV 尚未提供，简单状态也用 `sqlitedb`。
