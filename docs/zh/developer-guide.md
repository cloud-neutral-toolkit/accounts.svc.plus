# 开发手册

本页是共享双语工程参考页的中文导航层。

## 建议起步顺序

1. [代码结构参考](../development/code-structure.md)
2. [API 总览](../api/overview.md)
3. [认证与鉴权](../api/auth.md)
4. [接口矩阵](../api/endpoints.md)
5. [错误约定](../api/errors.md)
6. [测试基线](../development/testing.md)

## 每个细页回答什么问题

- [代码结构参考](../development/code-structure.md)
  看核心包由谁负责、哪些导出类型重要、哪些非导出 owner 承担主流程，以及各包之间如何连接。
- [API 总览](../api/overview.md)
  看路由族如何划分、handler 分布在哪些文件里、认证层是怎样叠加的。
- [认证与鉴权](../api/auth.md)
  看 session 登录、MFA、OAuth exchange、JWT refresh、密码重置、XWorkmate secret API 的请求/返回字段。
- [接口矩阵](../api/endpoints.md)
  看方法、路径、owner file、鉴权方式、请求参数、返回形状与主要依赖对象。
- [测试基线](../development/testing.md)
  看当前文档与代码一致性的校验方式，核心基线是 `go test ./...`。

## 校验基线

本轮文档补齐以源码为事实源，并通过以下命令校验：

```bash
go test ./...
```

如果你修改了路由、类型签名、handler 归属或主依赖对象，应同步更新上面的细页。

## 相关页面

- [架构](architecture.md)
- [设计](design.md)
- [环境搭建](../development/dev-setup.md)
- [贡献约定](../development/contributing.md)
