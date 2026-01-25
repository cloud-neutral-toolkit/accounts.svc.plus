# Gmail SMTP Setup Guide for Cloud Run

本文档详细说明如何配置 accounts.svc.plus 服务以使用 Gmail SMTP 发送邮件，并部署到 Google Cloud Run。

## 1. 获取 Gmail 应用专用密码 (App Password)

由于安全原因，您**不能**直接使用 Gmail 的登录密码。必须生成一个“应用专用密码”。

1.  登录您的 Google 账号。
2.  前往 **安全性 (Security)** 设置页面：[https://myaccount.google.com/security](https://myaccount.google.com/security)
3.  确保您已开启 **两步验证 (2-Step Verification)**（如果未开启，必须先开启）。
4.  在“两步验证”设置下方（或搜索栏搜索），找到 **应用专用密码 (App passwords)**。
    > **注意**：如果找不到此选项，可能是因为两步验证未开启。
5.  创建一个新密码：
    *   **应用 (App)** 选择 "Mail" (邮件)。
    *   **设备 (Device)** 选择 "Other" (其他)，填入名称如 `svc-plus-smtp`。
6.  点击 **生成 (Generate)**。
7.  复制生成的 **16位字符密码**（去掉空格）。这将是您的 `SMTP_PASSWORD`。

---

## 2. 配置 Google Cloud Secret Manager

为了安全地在 Cloud Run 中使用凭据，我们需要创建两个独立的 Secret：`smtp-username` 和 `smtp-password`。

请在您的 GCP 终端或者 Cloud Shell 中运行以下命令（请替换为您的真实信息）：

### 2.1 创建 SMTP 用户名 Secret
该 Secret 存储您的发件人邮箱地址。

```bash
# 替换 admin@svc.plus 为您的真实发信邮箱
gcloud secrets create smtp-username --replication-policy="automatic"

# 添加版本
printf "admin@svc.plus" | gcloud secrets versions add smtp-username --data-file=-
```

### 2.2 创建 SMTP 密码 Secret
该 Secret 存储步骤 1 中生成的 16 位应用专用密码。

```bash
# 替换 xxxx xxxx xxxx xxxx 为您的 16 位 Google 应用密码
gcloud secrets create smtp-password --replication-policy="automatic"

# 添加版本
printf "xxxx xxxx xxxx xxxx" | gcloud secrets versions add smtp-password --data-file=-
```

---

## 3. 绑定 svc.plus 域名

如果您希望发件人显示为 `@svc.plus` 后缀（如 `admin@svc.plus`），有两种情况：

### 情况 A：使用 Google Workspace (企业邮箱)
*   **适用场景**：您的 Google 账号本身就是 `admin@svc.plus`，且域名托管在 Google Workspace。
*   **配置方式**：
    *   **SMTP Host**: `smtp.gmail.com`
    *   **SMTP Username**: 填入您的完整企业邮箱，例如 `admin@svc.plus`。
    *   **SMTP Password**: 使用该账号生成的**应用专用密码**。
*   此方式最稳定、专业，推荐使用。

### 情况 B：使用个人 Gmail 代发
*   **适用场景**：您持有普通 Gmail 账号 (例如 `shenlan@gmail.com`)，但拥有 `svc.plus` 域名。
*   **配置方式**：
    1.  在 Gmail 网页端设置 -> 账号和导入 -> "发送邮件为" 中，添加另一个电子邮件地址。
    2.  输入您的域名邮箱（如 `no-reply@svc.plus`）。
    3.  配置 SMTP 服务器（通常使用 Gmail 的 SMTP）。
    4.  验证域名所有权（输入发送到域名邮箱的验证码）。
*   **风险**：接收方可能会看到 "由 gmail.com 代发" 的提示。

### 情况 C：直接使用个人 Gmail (无域名)
*   **适用场景**：您只想用个人的 `gmail.com` 邮箱发送通知，不强制要求域名。
*   **配置方式**：
    *   **SMTP Username**: 您的完整 Gmail 地址 (例如 `yourname@gmail.com`)。
    *   **SMTP Password**: 您的 16 位**应用专用密码**。
    *   **SMTP From**: 设置为 `XControl <yourname@gmail.com>`。
    *   **Cloud Run Env**: 将 `SMTP_FROM` 环境变量修改为您的 Gmail 地址。

---

## 4. 部署到 Cloud Run

确保您的 `deploy/gcp/cloud-run/service.yaml` 已包含以下配置（已在代码库中更新）：

```yaml
        # --- SMTP Configuration ---
        - name: SMTP_HOST
          value: "smtp.gmail.com"
        - name: SMTP_PORT
          value: "587"
        - name: SMTP_FROM
          value: "XControl Account <no-reply@svc.plus>"
        - name: SMTP_USERNAME
          valueFrom:
            secretKeyRef:
              name: smtp-username
              key: latest
        - name: SMTP_PASSWORD
          valueFrom:
            secretKeyRef:
              name: smtp-password
              key: latest
```

执行部署命令：

```bash
make cloudrun-deploy
```

部署完成后，Cloud Run 实例将自动读取 Secrets 作为环境变量，服务即可使用 Gmail SMTP 发送验证邮件。
