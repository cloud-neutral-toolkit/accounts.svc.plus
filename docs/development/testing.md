# 测试策略

## 单元测试

```bash
make test
# 或
go test ./...
```

## 集成测试

```bash
make integration-test
```

集成测试会执行初始化与创建管理员流程，依赖本地数据库环境。
