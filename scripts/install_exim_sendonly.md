
Exim Send-Only SMTP 一键脚本使用指南

环境假设：
系统：Ubuntu Server (22.04 / 24.04)
IP：52.196.108.28
域名：smtp.svc.plus
邮件地址：no-reply@svc.plus
证书：/etc/ssl/svc.plus.pem 与 /etc/ssl/svc.plus.key

🧱 一、首次部署（deploy）
sudo ./install_exim_sendonly.sh deploy

执行内容：

自动安装依赖包：exim4, opendkim, sasl2-bin, mailutils, dnsutils

配置：

TLS 启用 /etc/ssl/svc.plus.pem / /etc/ssl/svc.plus.key

SASL 认证（LOGIN / PLAIN）

DKIM 签名（自动生成私钥）

启动服务：exim4 + opendkim

输出结果：

🌐 推荐 DNS 配置模板：
----------------------------------------------------------
A     smtp.svc.plus       52.196.108.28
MX    svc.plus            smtp.svc.plus.
SPF   @                   "v=spf1 a:smtp.svc.plus -all"
DKIM  mail._domainkey     "v=DKIM1; k=rsa; p=MIIBIjANBg..."
DMARC _dmarc              "v=DMARC1; p=quarantine; rua=mailto:postmaster@svc.plus"
----------------------------------------------------------

📦 应用端 SMTP 配置（YAML）：
----------------------------------------------------------
smtp:
  host: "smtp.svc.plus"
  port: 587
  username: "no-reply@svc.plus"
  password: "YOUR_PASSWORD"
  from: "XControl Account <no-reply@svc.plus>"
  replyTo: ""
  timeout: 10s
  tls:
    mode: "auto"
    insecureSkipVerify: false
----------------------------------------------------------

🔍 二、自检发信环境（check self）
sudo ./install_exim_sendonly.sh check self

输出示例：
🔍 自检 SPF / DKIM / DMARC / rDNS / HELO ...

SPF:
"v=spf1 a:smtp.svc.plus -all"

DKIM:
"v=DKIM1; k=rsa; p=MIIBIjANBg..."

DMARC:
"v=DMARC1; p=quarantine; rua=mailto:postmaster@svc.plus"

rDNS:
smtp.svc.plus.

HELO 检查:
250-smtp.svc.plus Hello test
250-AUTH PLAIN LOGIN
250 STARTTLS


🧭 若以上 5 项全部正常 → 代表发信链路可信，邮件不易进垃圾箱。

📧 三、发送测试邮件（check send_email）
sudo ./install_exim_sendonly.sh check send_email you@gmail.com

输出：
📧 发送测试邮件到 you@gmail.com...
✅ 邮件已发送，请检查收件箱或垃圾邮件文件夹。
查看日志: sudo tail -n 20 /var/log/exim4/mainlog


邮件标题为：

✅ Exim Send-Only + DKIM + SASL Test

若在 Gmail / Outlook 查看邮件头，能看到：

Authentication-Results: dkim=pass spf=pass dmarc=pass

♻️ 四、更新配置或证书（upgrade）

当你更新 /etc/ssl/svc.plus.pem 或修改 DKIM / 域名时：

sudo ./install_exim_sendonly.sh upgrade


脚本会：

自动刷新 update-exim4.conf

重启服务

输出当前 DKIM / YAML 配置

📜 五、查看配置信息（show）
sudo ./install_exim_sendonly.sh show dns_record
sudo ./install_exim_sendonly.sh show app_config

示例：
🌐 推荐 DNS 配置模板：
A     smtp.svc.plus      52.196.108.28
MX    svc.plus           smtp.svc.plus.
SPF   @                  "v=spf1 a:smtp.svc.plus -all"
...

📦 应用端 SMTP 配置（YAML）：
smtp:
  host: "smtp.svc.plus"
  port: 587
  username: "no-reply@svc.plus"
  password: "YOUR_PASSWORD"
  from: "XControl Account <no-reply@svc.plus>"

🧹 六、安全卸载（uninstall 或 reset）
sudo ./install_exim_sendonly.sh uninstall
# 或：
sudo ./install_exim_sendonly.sh reset

脚本行为：

停止服务：systemctl stop exim4 opendkim

删除：

/etc/exim4/

/etc/opendkim/

/var/spool/exim4/

/var/log/exim4/

/etc/email-addresses

/etc/default/opendkim

不删除 /etc/ssl/* 与现有证书

询问是否删除 DKIM 私钥目录：

是否删除 DKIM 私钥文件？(y/N): y
🔒 已删除 DKIM 密钥。
✅ 已彻底清理 Exim4 + DKIM + SASL 环境（证书未动，可重新 deploy）

🚀 七、重新部署

清理后重新执行：

sudo ./install_exim_sendonly.sh deploy


会重新生成所有配置与 DKIM 密钥，
确保一个全新、干净、可验证的发信节点。

🧠 八、日志与调试

常用命令：

sudo tail -f /var/log/exim4/mainlog
sudo journalctl -u opendkim.service -b --no-pager | tail -n 20
sudo openssl s_client -connect smtp.svc.plus:587 -starttls smtp


检查 DKIM：

sudo opendkim-testkey -d svc.plus -s mail -vvv

✅ 九、健康状态一览
项目	检查方式	期望结果
SPF	dig TXT svc.plus	v=spf1 a:smtp.svc.plus -all
DKIM	dig TXT mail._domainkey.svc.plus	v=DKIM1; k=rsa; p=...
DMARC	dig TXT _dmarc.svc.plus	v=DMARC1; p=quarantine; ...
rDNS	dig -x 52.196.108.28	smtp.svc.plus.
HELO	EHLO test	返回 250-smtp.svc.plus
端口	nc -zv smtp.svc.plus 587	open
日志	/var/log/exim4/mainlog	<= / => 投递正常
🔒 十、总结

install_exim_sendonly.sh v3.1
是一个可重入、幂等、安全的 SMTP 部署器，特点：

单脚本管理全生命周期

DKIM + SPF + DMARC 三重防护

支持 SASL 登录 / STARTTLS

自动输出应用配置 YAML

保留系统证书、避免误删

提供交互式清理选项
