# 性能与扩展性

## 数据库连接池

配置项：
- `store.maxOpenConns`
- `store.maxIdleConns`

合理设置以避免数据库过载或连接不足。

## 会话存储

- 当前会话保存在进程内存
- 高并发或多实例场景建议外部化（未来可扩展）

## Xray 同步

- `xray.sync.interval` 控制同步频率
- 同步过于频繁会增加 I/O 与重启开销

## 日志级别

- `log.level` 支持 `debug|info|warn|error`
- 生产环境建议使用 `info` 或 `warn`
