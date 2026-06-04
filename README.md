# s-ui-go Community Build

> 基于 S-UI / s-ui-go 开源项目整理的社区构建版本。
> A community build organized from the open-source S-UI / s-ui-go project.

## 中文

### 项目简介

本仓库面向 `yongwei9527-art/s-ui-go` 发布，重点整理：

- Windows、Linux、macOS 三端发布压缩包
- 中文 / English 双语 Web 面板体验
- Linux、Windows、macOS 安装与卸载脚本
- DNS 防泄漏基础能力
- TLS / ECH / WebSocket / TUIC / Hysteria2 订阅转换兼容性

> 本仓库是社区整理版本，非官方原版。生产环境使用前请自行评估风险，并及时修改默认账号、密码、端口和访问路径。

### 下载

请在 GitHub Releases 下载对应系统压缩包：

- Windows amd64：`s-ui-windows-amd64.zip`
- Linux amd64：`s-ui-linux-amd64.zip`
- macOS amd64：`s-ui-macos-amd64.zip`

Release 页面：<https://github.com/yongwei9527-art/s-ui-go/releases>

### 协议组合指南

如果你不确定应该选哪个协议，请先看：[S-UI 协议组合建议方案](PROTOCOL_GUIDE.md)。

快速建议：

- 自建 VPS、无 CDN：优先 `VLESS + Reality + TCP`
- 有域名、走 CDN：优先 `VLESS/Trojan + TLS + WebSocket`
- 弱网、移动网络、游戏/UDP：优先 `Hysteria2/TUIC + TLS`
- 本地软件代理：优先 `Mixed`
- Windows/macOS 全局代理：优先 `Tun + DNS 接管`
- 软路由/网关：优先 `TProxy + DNS 劫持`

### Linux VPS 快捷安装

推荐先升级系统和安装基础依赖，再运行安装脚本。

#### Debian / Ubuntu

```bash
sudo apt update -y && sudo apt upgrade -y && sudo apt install -y curl wget tar unzip && curl -fsSL https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/main/install.sh -o install.sh && sudo bash install.sh
```

#### CentOS / Rocky / AlmaLinux / Oracle Linux

```bash
sudo yum update -y && sudo yum install -y curl wget tar unzip && curl -fsSL https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/main/install.sh -o install.sh && sudo bash install.sh
```

#### Fedora

```bash
sudo dnf upgrade -y && sudo dnf install -y curl wget tar unzip && curl -fsSL https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/main/install.sh -o install.sh && sudo bash install.sh
```

#### Arch / Manjaro

```bash
sudo pacman -Syu --noconfirm curl wget tar unzip && curl -fsSL https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/main/install.sh -o install.sh && sudo bash install.sh
```

通用安装命令：

```bash
curl -fsSL https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/main/install.sh -o install.sh && sudo bash install.sh
```

### Windows 安装

1. 下载 `s-ui-windows-amd64.zip`。
2. 解压到目标目录。
3. 右键 `install-windows.bat`，选择“以管理员身份运行”。
4. 按提示设置面板端口、面板路径、订阅端口、订阅路径、管理员账号和密码。
5. 安装完成后使用 `s-ui-windows.bat` 管理服务。

默认访问地址（如果安装时没有修改）：

- 面板：`http://localhost:2095/app/`
- 订阅：`http://localhost:2096/sub/`

### macOS 安装

1. 下载 `s-ui-macos-amd64.zip`。
2. 解压后查看 `install-macos.sh`、`uninstall-macos.sh` 和 `com.s-ui.plist`。
3. 按实际系统权限执行安装脚本。

```bash
chmod +x install-macos.sh uninstall-macos.sh
sudo ./install-macos.sh
```

### 从源码构建

Windows PowerShell 下推荐在仓库根目录运行：

```powershell
.\build-windows.ps1 -System windows -Architecture amd64 -Package -NonInteractive
.\build-windows.ps1 -System linux -Architecture amd64 -NoCGO -Package -NonInteractive
.\build-windows.ps1 -System darwin -Architecture amd64 -NoCGO -Package -NonInteractive
```

生成的发布压缩包：

- `dist/s-ui-windows-amd64.zip`
- `dist/s-ui-linux-amd64.zip`
- `dist/s-ui-macos-amd64.zip`

常用参数：

- `-System windows|linux|darwin`：目标系统
- `-Architecture amd64|arm64|386|arm`：目标架构
- `-NoCGO`：关闭 CGO，适合跨系统构建 Linux/macOS 包
- `-SkipFrontend`：跳过前端构建；需要已经存在 `web/html/`，通常先完整构建过一次前端后再使用
- `-Package`：生成 `dist/*.zip` 压缩包
- `-NonInteractive`：构建完成后不等待输入，适合自动化
- `-ListCleanCandidates`：只列出本地可清理候选，不删除任何文件

脚本组织说明：

- 推荐维护和运行仓库根目录的 `build-windows.ps1`、`build-windows.bat`、`install-windows.bat`、`uninstall-windows.bat` 和 `s-ui-windows.bat`。
- `windows/` 目录中的同名脚本仅作旧路径兼容或参考保留，后续更新应优先同步到根目录脚本。
- `web/html/` 是从 `frontend/dist/` 复制来的 Go embed 前端产物，通常由构建脚本生成，不建议手动修改。

只查看本地生成文件清理候选：

```powershell
.\build-windows.ps1 -ListCleanCandidates
```

### 验证

```bash
go test ./...
```

前端构建：

```bash
cd frontend
npm install
npm run build
```

基础检查：

- Web 管理界面能启动
- 能登录后台
- 能创建入站 / 节点配置
- 能生成订阅链接
- DNS 防泄漏相关配置通过基础检查
- 重启服务后配置仍然生效

### 上传 GitHub 时保留 / 忽略

建议提交：

- Go 后端源码：`api`、`app`、`cmd`、`config`、`core`、`database`、`service`、`sub`、`util`、`web`
- 前端源码：`frontend/src`
- 安装脚本：`install.sh`、`install-linux.sh`、`install-macos.sh`、`install-windows.bat`
- 构建脚本：`build-windows.ps1`、`build-windows.bat`
- 测试文件：`*_test.go`
- 发布压缩包：`dist/*.zip`

不要提交：

- `frontend/node_modules/`
- `frontend/dist/`
- `web/html/`
- `dist/` 下的临时展开目录
- 本机运行数据：`db/`、`logs/`、`cert/`
- 本地二进制和备份：`*.exe`、`*.dll`、`*.bak-*`
- 本地配置：`.claude/`、`.vscode/`、`*.env`

### 致谢

感谢原 S-UI / s-ui-go 项目及贡献者，感谢 sing-box、Xray-core、Vue、Vite、Go 生态和所有测试反馈者。

### 许可证

本项目遵循原项目许可证。使用、修改和分发前，请确认上游项目和依赖的许可证要求。

---

## English

### Overview

This repository is a community build published for `yongwei9527-art/s-ui-go`. It focuses on:

- Release packages for Windows, Linux, and macOS
- Chinese / English web panel usability
- Linux, Windows, and macOS install scripts
- DNS leak guard groundwork
- Subscription conversion compatibility for TLS, ECH, WebSocket, TUIC, and Hysteria2

> This is a community-maintained build, not the official upstream release. Please review risks before production use and change default credentials, ports, and access paths immediately.

### Downloads

Download release packages from GitHub Releases:

- Windows amd64: `s-ui-windows-amd64.zip`
- Linux amd64: `s-ui-linux-amd64.zip`
- macOS amd64: `s-ui-macos-amd64.zip`

Releases: <https://github.com/yongwei9527-art/s-ui-go/releases>

### Protocol guide

If you are not sure which protocol to choose, read [S-UI Protocol Combination Guide](PROTOCOL_GUIDE.md) first.

Quick recommendations:

- Self-hosted VPS without CDN: `VLESS + Reality + TCP`
- Domain with CDN: `VLESS/Trojan + TLS + WebSocket`
- Weak network, mobile network, gaming/UDP: `Hysteria2/TUIC + TLS`
- Local app proxy: `Mixed`
- Windows/macOS system-wide proxy: `Tun + DNS hijack`
- Router/gateway transparent proxy: `TProxy + DNS hijack`

### Linux quick install

```bash
curl -fsSL https://raw.githubusercontent.com/yongwei9527-art/s-ui-go/main/install.sh -o install.sh && sudo bash install.sh
```

### Windows install

1. Download `s-ui-windows-amd64.zip`.
2. Extract it.
3. Run `install-windows.bat` as Administrator.
4. Use `s-ui-windows.bat` to manage the service.

Default URLs if unchanged during installation:

- Panel: `http://localhost:2095/app/`
- Subscription: `http://localhost:2096/sub/`

### macOS install

Download `s-ui-macos-amd64.zip`, extract it, then run:

```bash
chmod +x install-macos.sh uninstall-macos.sh
sudo ./install-macos.sh
```

### Build from source

Recommended commands from the repository root on Windows PowerShell:

```powershell
.\build-windows.ps1 -System windows -Architecture amd64 -Package -NonInteractive
.\build-windows.ps1 -System linux -Architecture amd64 -NoCGO -Package -NonInteractive
.\build-windows.ps1 -System darwin -Architecture amd64 -NoCGO -Package -NonInteractive
```

Generated release packages:

- `dist/s-ui-windows-amd64.zip`
- `dist/s-ui-linux-amd64.zip`
- `dist/s-ui-macos-amd64.zip`

Script organization:

- Prefer the root scripts: `build-windows.ps1`, `build-windows.bat`, `install-windows.bat`, `uninstall-windows.bat`, and `s-ui-windows.bat`.
- Files under `windows/` are kept for legacy-path compatibility or reference. Update the root scripts first.
- `-SkipFrontend` requires existing `web/html/` assets, usually from a previous full frontend build.
- `web/html/` is generated from `frontend/dist/` for Go embedding; do not edit it manually.

List local generated-file cleanup candidates without deleting anything:

```powershell
.\build-windows.ps1 -ListCleanCandidates
```

### Validation

```bash
go test ./...
```

Frontend build:

```bash
cd frontend
npm install
npm run build
```

### Credits

Thanks to the original S-UI / s-ui-go projects and contributors, sing-box, Xray-core, the Vue / Vite / Go ecosystem, and all community testers.

### License

This project follows the upstream project license. Please review upstream and dependency licenses before use, modification, or redistribution.
