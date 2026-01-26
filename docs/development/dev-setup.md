# 开发环境

## 依赖

- Go 1.25.1
- PostgreSQL（如使用 postgres store）
- 可选：`air` 热重载工具

## 初始化

```bash
make init-db
make build
```

## 运行

```bash
make start
# 或
make dev
```

## 热重载

`make dev` 会检测 `air`，未安装会提示安装链接。
