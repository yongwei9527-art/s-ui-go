# S-UI 在 Ubuntu VPS 的生产部署指南

本文面向 Ubuntu VPS 的**非容器化、systemd 托管**部署。目标是让程序以低权限用户运行；管理面板与订阅服务仅监听 `127.0.0.1`，由反向代理（可选）负责公网 HTTPS。本文不修改项目默认监听值，而是在首次初始化后通过管理界面保存运行设置。

> **部署前的边界**
>
> - 面板和订阅服务的项目默认端口分别为 `2095` 与 `2096`，默认监听空地址（所有网卡）。因此，必须先完成 UFW，再进行第一次启动；不要在防火墙开放 `2095/2096`。
> - 数据库是 SQLite，默认路径由 `SUI_DB_FOLDER` 决定；本指南固定为 `/opt/s-ui/db/s-ui.db`。
> - 首次管理员环境变量仅在数据库中尚无用户时使用。成功初始化后应从环境文件删除管理员密码并重启服务。
> - 以下命令假定使用有 sudo 权限的 Ubuntu 管理员账户。将示例域名、用户名和密码替换为自己的真实值。

## 1. 选择产物与安装最小依赖

### 直接使用已构建的 Linux 产物（推荐）

从**可信发布来源**下载与 VPS 架构匹配的项目产物，连同发布方提供的校验值一起验证。仓库中使用的可执行文件命名为：

| `uname -m` 输出 | 应使用的文件 |
| --- | --- |
| `x86_64` | `sui-linux-amd64` |
| `aarch64` | `sui-linux-arm64` |
| 其他 ARM 32 位 | `sui-linux-arm` |

下载产物后先执行：

```bash
uname -m
sha256sum /path/to/downloaded/sui-linux-amd64
```

仅在校验值与可信发布信息一致时继续。本文后续把确认过的产物称为 `sui`。

最小运行依赖：

```bash
sudo apt update
sudo apt install -y ca-certificates curl openssl sqlite3 ufw
```

`sqlite3` 用于一致性备份和完整性检查；程序本身运行时不依赖系统 Go 或 Node.js。

### 从当前源码构建（可选）

仓库已有 `build.sh`，它会执行前端依赖安装与构建，然后用 `go build` 生成当前目录下的 `sui`。构建机（可与 VPS 分离）需要：

```bash
sudo apt update
sudo apt install -y git build-essential nodejs npm golang-go
cd /path/to/s-ui-source
./build.sh
```

构建完成后只把生成的 `sui` 文件部署到 VPS；不要把 `node_modules`、源码工作树或构建账户凭据复制到 `/opt/s-ui`。

## 2. 创建受限服务用户和目录布局

创建没有登录 shell 的专用账户，并建立目录：

```bash
sudo useradd --system --home-dir /opt/s-ui --shell /usr/sbin/nologin --user-group s-ui

sudo install -d -o root -g root -m 0755 /opt/s-ui
sudo install -d -o s-ui -g s-ui -m 0750 /opt/s-ui/db /opt/s-ui/logs /opt/s-ui/cert
sudo install -d -o root -g root -m 0700 /opt/s-ui/backups
sudo install -d -o root -g s-ui -m 0750 /etc/s-ui
```

推荐布局：

```text
/opt/s-ui/
├── sui                 # root:root，0755；可执行文件
├── db/                 # s-ui:s-ui，0750；包含 s-ui.db（及 WAL/SHM）
├── logs/               # s-ui:s-ui，0750
├── cert/               # s-ui:s-ui，0750；仅在程序自身配置证书时使用
└── backups/            # root:root，0700；离线或 SQLite 一致性备份

/etc/s-ui/s-ui.env      # root:s-ui，0640；systemd 环境文件
/etc/systemd/system/s-ui.service
```

复制已经校验的可执行文件（以下以 amd64 为例）：

```bash
sudo install -o root -g root -m 0755 /path/to/downloaded/sui-linux-amd64 /opt/s-ui/sui
sudo -u s-ui /opt/s-ui/sui -v
```

## 3. 首次管理员变量与 systemd 服务

生成一个仅用于首次启动的强密码，例如：

```bash
openssl rand -base64 32
```

创建 `/etc/s-ui/s-ui.env`。`SUI_HOME` 是本服务单元使用的运维变量；当前程序实际使用的是 `SUI_DB_FOLDER`。请把示例密码替换为上一步输出，不要把它提交到 Git、shell 历史、工单或聊天记录中：

```bash
sudo tee /etc/s-ui/s-ui.env >/dev/null <<'EOF'
SUI_HOME=/opt/s-ui
SUI_DB_FOLDER=/opt/s-ui/db
SUI_DEBUG=false
SUI_INITIAL_ADMIN_USERNAME=admin
SUI_INITIAL_ADMIN_PASSWORD=replace-with-a-random-password
EOF
sudo chown root:s-ui /etc/s-ui/s-ui.env
sudo chmod 0640 /etc/s-ui/s-ui.env
```

创建受限 systemd 单元 `/etc/systemd/system/s-ui.service`：

```ini
[Unit]
Description=S-UI Proxy Panel
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=s-ui
Group=s-ui
EnvironmentFile=/etc/s-ui/s-ui.env
WorkingDirectory=/opt/s-ui
ExecStart=/opt/s-ui/sui
Restart=on-failure
RestartSec=10
UMask=0077

NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/opt/s-ui/db /opt/s-ui/logs /opt/s-ui/cert

[Install]
WantedBy=multi-user.target
```

安装、加载并启用服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now s-ui
sudo systemctl status s-ui --no-pager
sudo journalctl -u s-ui -n 100 --no-pager
```

如果启动失败，先检查 `/etc/s-ui/s-ui.env` 是否存在空的 `SUI_INITIAL_ADMIN_PASSWORD`。程序会拒绝使用显式空密码创建管理员。

首次登录确认成功后，删除**两行**首次管理员变量，保留其它变量：

```bash
sudo sed -i '/^SUI_INITIAL_ADMIN_USERNAME=/d; /^SUI_INITIAL_ADMIN_PASSWORD=/d' /etc/s-ui/s-ui.env
sudo systemctl restart s-ui
```

> 仅当数据库中还没有用户时，这两个变量才生效；删除它们不会修改已创建的管理员账户。

## 4. 先收紧网络，再将监听地址设为 127.0.0.1

在首次访问前先启用 UFW。若当前 SSH 不是 22 端口，先把下面的 `22` 改成实际端口，并确认保留当前 SSH 会话，避免将自己锁在 VPS 外：

```bash
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw enable
sudo ufw status verbose
```

此规则集**不开放** `2095` 或 `2096`。这样即使程序的首次数据库还保留项目默认的空监听地址，公网也无法访问它。

使用 SSH 本地转发访问刚启动的面板：

```bash
ssh -N -L 2095:127.0.0.1:2095 your-ssh-user@your-vps-ip
```

在本机浏览器访问 `http://127.0.0.1:2095/app/` 并登录。进入 **设置** 页面后：

1. 在“界面”页把“监听地址”设为 `127.0.0.1`，保留或按需修改面板端口与路径；
2. 在“订阅”页把“监听地址”也设为 `127.0.0.1`，保留或按需修改订阅端口与路径；
3. 保存设置，然后使用页面的“重启应用”操作，或执行 `sudo systemctl restart s-ui`。

重启后在 VPS 上确认仅有 loopback 监听：

```bash
sudo ss -ltnp | grep -E ':(2095|2096)\b'
sudo systemctl is-active s-ui
```

输出应显示 `127.0.0.1:2095` 和 `127.0.0.1:2096`，而不是 `0.0.0.0:*` 或 `[::]:*`。

## 5. 有域名时：可选的 Caddy TLS 反代

Caddy **不是强制要求**。若你拥有域名并希望把面板或订阅稳定地公开给互联网，建议使用任意可信 HTTPS 反代；以下仅给出 Caddy 示例。开始前让域名 A/AAAA 记录指向该 VPS，并保持 UFW 的 `80/443` 已放行。

安装 Caddy：

```bash
sudo apt update
sudo apt install -y caddy
```

例如面板域名为 `panel.example.com`、订阅域名为 `sub.example.com`，创建 `/etc/caddy/Caddyfile`：

```caddyfile
panel.example.com {
    reverse_proxy 127.0.0.1:2095
}

sub.example.com {
    reverse_proxy 127.0.0.1:2096
}
```

该配置保留原始路径，因此面板路径仍是 `/app/`、订阅路径仍是 `/sub/`，除非你已在 S-UI 设置页面改过它们。订阅实际请求还必须携带客户端对应的订阅 ID（即 `/sub/<your-subscription-id>`）。验证并加载：

```bash
sudo caddy validate --config /etc/caddy/Caddyfile
sudo systemctl enable --now caddy
sudo systemctl reload caddy
curl -fsSI https://panel.example.com/app/
curl -fsSI https://sub.example.com/sub/<your-subscription-id>
```

Caddy 终止 TLS 时，不要再为 S-UI 的 loopback 监听配置重复的 `webCertFile/webKeyFile` 或 `subCertFile/subKeyFile`；由 Caddy 对公网提供证书即可。将 S-UI 设置中的面板/订阅域名按实际公网域名填写，可便于它生成正确的地址。

## 6. 无域名安全模式（推荐于暂时没有域名）

没有域名时，**不要**把管理面板公开到公网。保持面板的 `webListen=127.0.0.1`，通过 SSH LocalForward 管理：

```bash
ssh -N -L 2095:127.0.0.1:2095 your-ssh-user@your-vps-ip
```

然后仅在自己的电脑上打开：

```text
http://127.0.0.1:2095/app/
```

订阅也应优先保持 `subListen=127.0.0.1`。不建议长期使用裸 HTTP、IP 地址加自签名证书，或让客户端忽略证书错误来进行公网订阅分发：订阅内容可能包含用户标识和连接配置，明文传输可被窥探或篡改；自签名/IP 证书无法提供稳定、可验证的公共身份。

### 若确实必须无域名公开订阅

这不是推荐的生产方案。最低限度应满足以下条件：

1. 面板仍始终保持 `127.0.0.1`；**只**将订阅监听地址改为公网地址，并明确知道这会使订阅端口接受网络连接；
2. 不要添加对全网开放的 `2096/tcp` UFW 规则；只允许已知、固定的客户端出口 IP，例如：

   ```bash
   sudo ufw allow from 203.0.113.10 to any port 2096 proto tcp
   sudo ufw status numbered
   ```

   将 `203.0.113.10` 换成真实的固定客户端 IP；移动网络、漫游或不固定 IP 的客户端不适用该策略；
3. 同时检查 VPS 服务商安全组/云防火墙，确保它也只允许相同来源和端口；
4. 明确接受裸 HTTP 的被动窃听、内容篡改、订阅泄漏与可用性风险。IP 自签名证书并不能替代由可信 CA 为域名签发的证书；
5. 一旦具备域名条件，迁移到“有域名时”的 HTTPS 反代方案，并删除该 `2096` 白名单规则：

   ```bash
   sudo ufw delete allow from 203.0.113.10 to any port 2096 proto tcp
   ```

## 7. 数据库备份、恢复前检查与日常验证

S-UI 使用 SQLite，并启用 WAL。**不要**在服务运行时只用 `cp` 复制 `s-ui.db`；应使用 SQLite 的 `.backup` 生成一致性快照：

```bash
sudo sqlite3 /opt/s-ui/db/s-ui.db ".backup '/opt/s-ui/backups/s-ui-$(date +%F-%H%M%S).db'"
sudo find /opt/s-ui/backups -maxdepth 1 -type f -name 's-ui-*.db' -printf '%f\n'
sudo sqlite3 /opt/s-ui/backups/s-ui-YYYY-MM-DD-HHMMSS.db 'PRAGMA integrity_check;'
```

最后一条应返回 `ok`。备份文件含有管理员数据、令牌、客户端与订阅配置，应维持目录 `0700`，并把加密后的副本传送到独立的受控存储。

每次部署或改动监听设置后，执行：

```bash
sudo systemctl is-active s-ui
sudo systemctl status s-ui --no-pager
sudo journalctl -u s-ui -n 100 --no-pager
sudo ss -ltnp | grep -E ':(2095|2096)\b'
sudo ufw status verbose
```

有域名且使用 Caddy 时，额外确认：

```bash
sudo systemctl is-active caddy
curl -fsSI https://panel.example.com/app/
curl -fsSI https://sub.example.com/sub/<your-subscription-id>
```

## 8. 上线前最小检查清单

- [ ] `s-ui` 以 `s-ui` 系统用户运行，而不是 root；
- [ ] `/etc/s-ui/s-ui.env` 为 `root:s-ui`、`0640`，且首次管理员密码变量已在首次登录后删除；
- [ ] 面板与订阅监听均确认是 `127.0.0.1`；
- [ ] UFW 和云防火墙只放行 SSH、`80/tcp`、`443/tcp`；没有全网开放 `2095/2096`；
- [ ] 有公网分发需求时使用已验证域名和可信 TLS；
- [ ] 已执行 SQLite `.backup`，并对备份运行 `PRAGMA integrity_check`；
- [ ] 已保存可用的 SSH 登录路径和离线备份，避免因反代或防火墙改动失去管理能力。