# Windows 后端搭建、首次启动与生产部署

本文适用于本仓库的 Windows 后端构建产物与 `install-windows.bat` 安装器。命令应在**以管理员身份**打开的 PowerShell 或命令提示符中执行；服务、端口、证书和程序目录均可能需要管理员权限。

> **安全结论先行：当前代码的默认监听并未改成 loopback。** 新数据库的面板默认是 `:2095/app/`，订阅默认是 `:2096/sub/`；空监听地址会由 Go 绑定到所有可用网络接口。请在连接公网或端口转发之前按本文的“生产部署”章节收紧监听与防火墙规则。

---

## 1. 构建 Windows 后端

### 1.1 前置条件

- Windows 10/11 或 Windows Server；建议使用 64 位系统。
- Go、Node.js（含 npm）均已加入 `PATH`。
- 从仓库根目录执行命令。
- 若使用 `install-windows.bat`，还需可访问 WinSW 的下载地址，或预先准备对应架构的 WinSW 文件。

### 1.2 标准构建

在仓库根目录执行：

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\build-windows.ps1 -System windows -Architecture amd64 -NonInteractive
```

该命令会构建前端、将前端资源嵌入后端，并生成：

```text
sui-windows-amd64.exe
sui.exe
```

需要生成可分发 ZIP 包时使用：

```powershell
.\build-windows.ps1 -System windows -Architecture amd64 -Package -NonInteractive
```

压缩包输出在：

```text
dist\s-ui-windows-amd64.zip
```

仅在已经存在有效 `frontend\dist\index.html` 且明确希望跳过前端构建时，才使用 `-SkipFrontend`：

```powershell
.\build-windows.ps1 -System windows -Architecture amd64 -SkipFrontend -NonInteractive
```

不要把未验证来源的 `sui.exe`、DLL、配置文件或数据库放入生产目录。

---

## 2. 安装器行为与首次安装

### 2.1 使用安装器

1. 解压 Windows 发布包到本地磁盘，例如 `C:\Temp\s-ui-release`。
2. 右键“以管理员身份运行” `install-windows.bat`。
3. 安装器会：
   - 询问并写入面板端口/路径、订阅端口/路径；直接回车时沿用 `2095`、`/app/`、`2096`、`/sub/`。
   - 询问管理员用户名和隐藏输入的密码；用户名留空时为 `admin`，密码留空会退出安装。
   - 执行 `sui.exe setting ...` 和 `sui.exe admin ...` 写入配置与管理员凭据。
   - 创建/安装 WinSW 服务（服务名 `s-ui`），设置为自动启动，并尝试启动服务。
   - 在安装目录写入 `s-ui-windows.env`，供控制脚本读取端口与路径。
   - 设置系统环境变量 `SUI_HOME` 指向安装目录，并创建控制快捷方式。

安装完成后，先在服务器本机访问：

```text
http://localhost:2095/app/
```

如果安装器提示服务启动失败，请先查看“日志与排障”章节，而不是立即把端口暴露到公网。

### 2.2 安装器不是安全网关

安装器当前只配置端口和路径，**不会自动把后端监听地址改为 `127.0.0.1`，不会申请公网 TLS 证书，也不会自动配置 Caddy/Nginx 或 Windows 防火墙**。这些应由部署人员按第 5 节完成。

---

## 3. 首次管理员凭据与环境变量

### 3.1 仅适用于“首次建库”

后端仅在数据库中尚无用户时读取以下变量：

| 变量 | 含义 | 未设置时 |
| --- | --- | --- |
| `SUI_INITIAL_ADMIN_USERNAME` | 首个管理员用户名 | 使用 `admin` |
| `SUI_INITIAL_ADMIN_PASSWORD` | 首个管理员密码 | 生成随机密码 |

它们**只在首次创建空数据库时有效**。数据库中已有用户后，即使再修改环境变量，也不会重置、覆盖或恢复管理员凭据。需要修改已有管理员时，应通过面板或在停止服务后使用：

```powershell
cd "C:\Program Files\s-ui"
.\sui.exe admin -username "新用户名" -password "高强度新密码"
```

> 上述目录是常见安装目录；请按实际 `SUI_HOME` 或安装路径调整。

### 3.2 推荐的无人值守首次启动

对于手动部署或自动化部署，在**第一次启动服务之前**设置两个变量，且不要把密码写进版本库、镜像层、命令历史或共享脚本：

```powershell
[Environment]::SetEnvironmentVariable('SUI_INITIAL_ADMIN_USERNAME', 'ops-admin', 'Machine')
[Environment]::SetEnvironmentVariable('SUI_INITIAL_ADMIN_PASSWORD', '请替换为随机生成的长密码', 'Machine')
```

设置为机器环境变量后，重新打开管理员终端或重启服务，以确保服务进程读到新环境。推荐使用密码库、部署平台的 Secret 或受限 ACL 的服务环境配置保存密码。

### 3.3 未设置密码时的行为

若首次建库时**没有设置** `SUI_INITIAL_ADMIN_PASSWORD`：

- 后端会生成一个一次性的随机管理员密码；
- 密码仅在该次首次启动时输出到**进程标准错误（stderr）**；
- 请立即从服务控制台、WinSW 日志或受控的启动输出中保存该密码，并在首次登录后更换；
- 这不是“每次启动重新生成密码”，也不会在后续启动再次输出。

若变量存在但值为空（例如 `SUI_INITIAL_ADMIN_PASSWORD=`），后端会拒绝初始化并拒绝启动，以避免创建空密码账户。请设置非空强密码后再启动；不要通过删除数据库来绕过此保护。

Windows 安装器会主动要求输入非空管理员密码并随后调用 `sui.exe admin`，因此交互式安装优先使用安装器提示的凭据；环境变量主要适用于手动/自动化的首次建库流程。

---

## 4. 当前默认监听与公网风险

当前新数据库的默认设置如下（本文仅说明现状，**没有改变这些默认值**）：

| 服务 | 默认监听 | 默认路径 | 默认协议 | 风险 |
| --- | --- | --- | --- | --- |
| 管理面板 | `:2095`（空 `webListen`） | `/app/` | HTTP | 可能从所有网络接口访问，登录 Cookie 和管理操作会以明文 HTTP 传输。 |
| 订阅服务 | `:2096`（空 `subListen`） | `/sub/` | HTTP | 订阅 URL、令牌和节点内容可能被窃听或篡改。 |

`/app/` 和 `/sub/` 是路径，不是访问控制措施；更换端口或路径也不能替代 TLS、认证、主机防火墙和最小网络暴露。

在完成反向代理和 TLS 前：

1. 不要在云安全组、路由器端口转发或 Windows 防火墙中对公网放行 `2095`/`2096`。
2. 不要将 `http://服务器公网IP:2095/app/` 作为日常管理入口。
3. 不要把 HTTP 订阅地址分发给用户。
4. 不要将内部自签名/临时入站配置直接作为公网生产节点；请配置可验证的域名、证书和必要的访问限制。

可先检查本机监听：

```powershell
Get-NetTCPConnection -State Listen | Where-Object { $_.LocalPort -in 2095,2096 } |
    Format-Table LocalAddress,LocalPort,OwningProcess -AutoSize
```

---

## 5. 推荐生产部署：loopback 后端 + TLS 反向代理

### 5.1 目标拓扑

推荐将面板和订阅后端都限制为本机 loopback，由反向代理统一对公网提供 HTTPS：

```text
Internet
  ├─ https://panel.example.com/app/  ─┐
  └─ https://sub.example.com/sub/     ├─ Caddy 或 Nginx（公网 80/443，TLS）
                                      └─ S-UI 仅监听 127.0.0.1:2095 / 127.0.0.1:2096
```

在面板的系统设置中完成以下配置并重启服务：

- `webListen`：`127.0.0.1`
- `webPort`：`2095`（或其他本机端口）
- `webPath`：`/app/`
- `subListen`：`127.0.0.1`
- `subPort`：`2096`（或其他本机端口）
- `subPath`：`/sub/`
- `webDomain`：`panel.example.com`
- `subDomain`：`sub.example.com`
- `subURI`：`https://sub.example.com/sub/`（推荐显式设置为公网 HTTPS 地址）

当前命令行 `sui.exe setting` 仅直接提供端口和路径参数；监听地址、域名、证书相关字段应在受保护的面板系统设置中配置。变更前请保留本机控制台访问方式，避免把自己锁在面板外。

随后仅在 Windows 防火墙/云安全组中对公网放行反向代理使用的 `TCP 80`（证书签发/跳转需要时）与 `TCP 443`。不要放行 `2095`、`2096`。

### 5.2 Caddy 示例（推荐）

确保 `panel.example.com` 和 `sub.example.com` 的 A/AAAA 记录都指向此服务器，并确认 80/443 可从公网访问。创建 `Caddyfile`：

```caddyfile
panel.example.com {
    reverse_proxy 127.0.0.1:2095
}

sub.example.com {
    reverse_proxy 127.0.0.1:2096
}
```

Caddy 会自动申请和续期公网 TLS 证书。配置后验证：

```powershell
curl.exe -I https://panel.example.com/app/
curl.exe -I https://sub.example.com/sub/
```

应始终向用户分发 `https://sub.example.com/sub/` 下的实际订阅链接，而不是 `http://IP:2096/...`。反向代理终止 TLS 时，S-UI 后端本身保持 HTTP loopback 即可；不要同时在同一后端端口重复配置不匹配的证书。

### 5.3 Nginx 示例

已在服务器上部署 Nginx 和证书（例如 Certbot）时，可使用如下配置；证书路径请替换为实际值：

```nginx
server {
    listen 80;
    server_name panel.example.com sub.example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name panel.example.com;

    ssl_certificate     C:/nginx/certs/panel/fullchain.pem;
    ssl_certificate_key C:/nginx/certs/panel/privkey.pem;

    location /app/ {
        proxy_pass http://127.0.0.1:2095;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}

server {
    listen 443 ssl http2;
    server_name sub.example.com;

    ssl_certificate     C:/nginx/certs/sub/fullchain.pem;
    ssl_certificate_key C:/nginx/certs/sub/privkey.pem;

    location /sub/ {
        proxy_pass http://127.0.0.1:2096;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
    }
}
```

检查并重载：

```powershell
nginx.exe -t
nginx.exe -s reload
```

面板域名与订阅域名不同是推荐实践；面板域名还应配合 VPN、IP 白名单或额外身份认证，降低暴力破解和管理面暴露风险。

---

## 6. 服务管理、日志与排障

### 6.1 服务管理

常用管理员命令：

```powershell
sc.exe query s-ui
net stop s-ui
net start s-ui
Restart-Service -Name s-ui
```

安装目录中的 `s-ui-windows.bat` 也提供服务状态、日志目录和本机打开面板等操作。

### 6.2 日志位置与敏感性

WinSW 服务日志默认在安装目录的：

```text
<安装目录>\logs\
```

WinSW 配置为单文件约 10 MB、最多保留 10 个轮转日志。首次随机管理员密码（未设置 `SUI_INITIAL_ADMIN_PASSWORD` 时）会写入首次进程的 stderr，因此可能出现在此目录中的服务错误/输出日志。日志应视为敏感数据：

- 不要上传到公开工单、聊天群、截图或无访问控制的日志平台；先脱敏 URL、令牌、用户名、IP 和密码。
- 仅授予管理员和实际运行服务账户读取权限。
- 设置日志保留与备份策略，定期删除超过保留期的历史日志。
- 出现启动失败时，先核对 WinSW 日志、Windows 事件查看器及端口占用，再决定是否重启。

### 6.3 常见检查

```powershell
# 查看服务状态
sc.exe query s-ui

# 查看端口占用
Get-NetTCPConnection -State Listen | Where-Object { $_.LocalPort -in 2095,2096,80,443 } |
    Format-Table LocalAddress,LocalPort,OwningProcess -AutoSize

# 查看当前面板/订阅端口与路径（在安装目录运行）
.\sui.exe setting -show

# 查看管理员用户名（不会显示密码）
.\sui.exe admin -show
```

---

## 7. 数据库备份、恢复与文件权限

### 7.1 数据库位置与备份

默认数据库路径由 `SUI_DB_FOLDER` 决定：未设置时为可执行文件同级的 `db` 目录，数据库文件通常为：

```text
<安装目录>\db\s-ui.db
```

先确认实际路径：

```powershell
$env:SUI_DB_FOLDER
```

若服务环境设置了 `SUI_DB_FOLDER=db`，它相对于服务工作目录解析，通常仍对应 `<安装目录>\db`。数据库中可能包含用户、会话密钥、API 令牌、订阅、客户端及节点配置，必须按秘密材料处理。

建议至少每日进行一次离机/异地加密备份。为避免 SQLite 在写入时产生不一致副本，请先停止服务后复制数据库及可能存在的 WAL/SHM 文件：

```powershell
$install = 'C:\Program Files\s-ui'       # 改为实际安装目录
$backup = 'D:\SecureBackups\s-ui'        # 必须是受限、加密的位置
Stop-Service -Name s-ui
New-Item -ItemType Directory -Force -Path $backup | Out-Null
Copy-Item "$install\db\s-ui.db*" -Destination $backup -Force
Start-Service -Name s-ui
```

恢复前同样应先停止服务，先将当前数据库完整复制到安全位置，再以受信任的备份替换。恢复后检查文件 ACL，启动服务并从本机/反向代理健康检查确认数据正确。不要从不可信来源导入数据库或配置备份。

### 7.2 权限基线

安装器会尝试设置程序目录权限，但生产环境仍应人工核验，尤其是 `db`、`logs`、`cert` 目录。它们不应对普通本地用户具有写入权限；`db` 和私钥目录不应对普通用户具有读取权限。

以下示例将应用程序目录的管理员/系统权限和普通用户读取执行权限分开，并将数据库、日志、证书目录限制为管理员与 LocalSystem。执行前请确认 WinSW 服务账户；若你改用自定义服务账户，必须额外授予该账户所需权限。

```powershell
$install = 'C:\Program Files\s-ui'  # 改为实际安装目录

icacls $install /inheritance:r /grant:r `
  '*S-1-5-32-544:(OI)(CI)F' `
  '*S-1-5-18:(OI)(CI)F' `
  '*S-1-5-32-545:(OI)(CI)RX'

foreach ($dir in 'db','logs','cert') {
    $path = Join-Path $install $dir
    if (Test-Path -LiteralPath $path) {
        icacls $path /inheritance:r /grant:r `
          '*S-1-5-32-544:(OI)(CI)F' `
          '*S-1-5-18:(OI)(CI)F'
    }
}
```

其中 `S-1-5-32-544` 是本地 Administrators，`S-1-5-18` 是 LocalSystem，`S-1-5-32-545` 是 Users；采用 SID 是为了避免 Windows 显示语言差异。执行后检查：

```powershell
icacls 'C:\Program Files\s-ui'
icacls 'C:\Program Files\s-ui\db'
icacls 'C:\Program Files\s-ui\logs'
icacls 'C:\Program Files\s-ui\cert'
```

如果服务并非以 LocalSystem 运行，直接套用上述命令可能导致服务无法读写数据库或日志。应先在“服务”管理器或 `sc.exe qc s-ui` 中确认账户，再为该专用服务账户授予最小权限。

---

## 8. 上线前检查清单

- [ ] 首次管理员密码已设置为非空高强度密码，且已保存在受控密码库。
- [ ] 已确认环境变量只用于首次空数据库初始化，未误认为可以在后续启动重置密码。
- [ ] `webListen` 与 `subListen` 已设为 `127.0.0.1`，并已重启验证。
- [ ] Caddy/Nginx 已使用有效公网证书；面板和订阅均通过 HTTPS 访问。
- [ ] Windows 防火墙和云安全组只对公网开放必要的 `80/443`；`2095/2096` 未公开。
- [ ] `subURI` 指向正确的 `https://` 公网订阅地址。
- [ ] 数据库、私钥、日志与备份均采用最小权限和加密存储。
- [ ] 已完成一次可验证的备份与恢复演练。
- [ ] 已在外部网络验证 TLS 证书、面板登录、订阅获取和服务重启恢复。
