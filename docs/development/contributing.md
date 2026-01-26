# 贡献指南

欢迎贡献本项目。建议流程：

1. 阅读 `docs/development/dev-setup.md`
2. 创建分支并保持提交粒度清晰
3. 运行 `go test ./...`
4. 确保文档与配置同步更新

如需调整数据库结构，请同步更新：
- `sql/schema.sql`
- 相关迁移与测试
