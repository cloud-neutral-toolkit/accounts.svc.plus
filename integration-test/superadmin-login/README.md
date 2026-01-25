# superadmin-login

## 说明

该用例用于本地集成测试，按顺序执行：

1. `make init-db`
2. `make create-db-user`
3. `make create-super-admin`

脚本会自动读取项目根目录 `.env` 中的环境变量（如 `POSTGRES_USER` / `POSTGRES_PASSWORD`），用于联动数据库配置。
若未设置 `SUPERADMIN_PASSWORD`，脚本会生成随机密码并写回 `.env`，便于后续登录测试复用。

## 运行方式

```bash
make integration-test
```

或直接运行脚本：

```bash
bash integration-test/superadmin-login/run-test-scripts.sh
```

## API 自动化测试

```bash
bash integration-test/superadmin-login/api-test.sh
```

可选环境变量：

- `API_BASE_URL`：API 入口地址（默认 `https://accounts.svc.plus`）
- `LOGIN_EMAIL`：登录邮箱（默认 `admin@svc.plus`）
- `SUPERADMIN_PASSWORD`：登录密码（从 `.env` 读取）

## UI 自动化测试（Playwright）

```bash
bash integration-test/superadmin-login/ui-test.sh
```

可选环境变量：

- `UI_BASE_URL`：UI 入口地址（默认 `https://console.svc.plus`）
- `LOGIN_EMAIL`：登录邮箱（默认 `admin@svc.plus`）
- `SUPERADMIN_PASSWORD`：登录密码（从 `.env` 读取）

## 预期输出（示例）

```
✅ 已读取 .env
>>> init-db
...（略）
>>> create-super-admin
...（略）
✅ 集成测试步骤完成（DB 初始化 + 用户创建 + 超级管理员创建）

接下来手动登录验证：
- 网址：https://console.svc.plus/login
- 邮箱：admin@svc.plus
- 密码：<generated-or-provided>
```

> 注意：登录验证需要手动在浏览器完成。
