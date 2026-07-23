# s-ui-go 0.1 在 Ubuntu VPS 的部署指南

> **用户可见名称：** s-ui-go 0.1
> **发布标签：** [`s-ui-go-v0.1.0`](https://github.com/yongwei9527-art/s-ui-go/releases/tag/s-ui-go-v0.1.0)

本文说明 Ubuntu VPS 的非容器化安装。受支持的包内安装方式要求 **systemd**；默认安装目录为 `/usr/local/s-ui`。本文中的 `sui` 是程序文件名，`s-ui` 是 systemd 服务与管理命令。

## 1. 部署边界与前置条件

- 使用带 systemd 的 Ubuntu（例如 22.04/24.04），并以具有 `sudo` 权限的账户执行；不要在没有 systemd 的容器、精简 chroot 或 WSL 环境中按本指南部署。
- 本节使用 amd64 发布资产，先确认 `uname -m` 为 `x86_64`。其他架构只能使用同一 Release 中实际发布的匹配 ZIP 与校验文件。
- Linux 的发布契约是 **`s-ui-linux-amd64.zip` + `s-ui-linux-amd64.zip.sha256`**，不是单独的 `sui-linux-amd64` 裸二进制。
- 云安全组/云防火墙不受 VPS 内安装器控制；即使本机服务已监听，仍须由你在云平台确认入站规则。
- 管理面板与订阅端口默认为 `2095` / `2096`。安装器不会承诺或要求自动对公网开放它们；生产环境应优先限制监听地址并经 HTTPS 反代或 SSH 隧道管理。

确认 systemd：

```bash
systemctl --version
systemctl is-system-running --wait || true
```

## 2. 从固定 Release 下载并校验 ZIP

先安装依赖：

```bash
sudo apt update
sudo apt install -y ca-certificates curl unzip openssl iproute2 coreutils
```

不要从任何可变分支或通过 `curl | bash` 建立 bootstrap。请只信任固定 Release 的 ZIP 及其同名校验资产：

```bash
TAG='s-ui-go-v0.1.0'
ASSET='s-ui-linux-amd64.zip'
BASE="https://github.com/yongwei9527-art/s-ui-go/releases/download/${TAG}"

mkdir -p ~/s-ui-go-install && cd ~/s-ui-go-install
curl -fL -O "${BASE}/${ASSET}"
curl -fL -O "${BASE}/${ASSET}.sha256"
sha256sum --check "${ASSET}.sha256"
unzip -t "${ASSET}"
rm -rf package && unzip -q "${ASSET}" -d package
cd package
ls -l sui install-linux.sh XRAY-CORE-NOTICE.md
```

只有 `sha256sum --check` 输出 `OK` 且 `unzip -t` 成功时才继续。已验证的压缩包内含 `sui`、`install-linux.sh`、服务管理文件和合规说明；不会有独立 `xray` 文件。

## 3. 使用包内安装器

在上一步解压出的 `package` 目录执行：

```bash
sudo bash ./install-linux.sh
```

此安装路径固定为“**校验 ZIP → 解压 → 使用包内 `sui` / `install-linux.sh`**”。安装器将程序放到 `/usr/local/s-ui` 并由 systemd 管理 `s-ui.service`。

### 全新安装的严格检查与回滚

全新安装会在创建可用配置前检查 Reality 目标、端口占用和双核心实际启动状态：

| 核心 | 默认入站 | 默认端口 |
| --- | --- | --- |
| sing-box | VLESS Reality | `443/tcp` |
| 内置 Xray-core | VLESS Reality TCP Vision | `10443/tcp` |
| 内置 Xray-core | VLESS Reality XHTTP | `2088/tcp` |

检查通过后，安装器会启用并确认三个监听。检查、安装或 systemd 启动任一步失败时，安装器会中止并回滚本次全新安装产生的状态，不把“仅面板启动”误报为双核心可用。

已有数据库的升级不会擅自重置管理员、面板设置或现有协议配置；安装器会备份升级前状态，在失败时恢复。`--allow-manual-xray` 仅适合明确接受 Xray 默认模板暂时关闭的高级人工部署，不能视为双核心默认可用的成功结果。

## 4. 防火墙、云安全组与面板暴露

安装成功只表示 VPS 本机的配置与监听检查通过，不代表公网已可用。请在**本机防火墙和云安全组**中分别确认实际需要的端口：

```bash
# 仅在你确认需要将代理端口对公网开放时执行；按实际安全策略限制来源。
sudo ufw allow 443/tcp
sudo ufw allow 10443/tcp
sudo ufw allow 2088/tcp
sudo ufw status verbose
```

- 不要把 `2095/tcp`、`2096/tcp` 作为“安装必需端口”对所有来源开放。推荐将 Web 面板和订阅监听限制到 `127.0.0.1`，再通过可信 HTTPS 反代或 SSH 本地转发访问。
- 如果本机 UFW 已启用，包内安装器只会针对通过严格检查的代理端口采取相应动作；它不会替你配置云安全组、也不替你承诺面板/订阅端口对公网可达。
- 新增或变更端口后，必须同时检查 UFW（或其他本机防火墙）与云厂商安全组/网络 ACL。

## 5. Reality 的非 443 端口风险

默认 sing-box Reality 使用 `443/tcp`，但内置 Xray TCP Vision 与 XHTTP 分别使用 `10443/tcp` 和 `2088/tcp`。后二者都是非 443 Reality 入站。

[Xray 官方 Reality/传输配置文档](https://xtls.github.io/en/config/transport.html) 提示：当 Reality 服务使用非 443 端口时，Reality 目标站可能对相关来源施加封锁。默认 `10443`/`2088` 并非无风险伪装。部署前应在自己的网络条件下评估以下选择：

1. 让 Reality 服务与访问目标的路径使用不同 IP（降低相互关联的风险）；
2. 使用适当的端口映射；
3. 明确接受该封锁风险；或
4. 不能接受风险时，不启用这两个默认非 443 入站。

不要把 Reality 放在 CDN 后方。更改 Reality 目标、`serverName`、`dest`、密钥、short ID 或端口后，应重新进行实际连通性测试。

## 6. 验证、运行与客户端范围

安装后检查：

```bash
sudo systemctl status s-ui --no-pager
sudo journalctl -u s-ui -n 100 --no-pager
sudo ss -ltnp | grep -E ':(443|10443|2088)\b'
sudo /usr/local/s-ui/sui -v
```

Xray-core `v26.3.27`（Go 模块 `github.com/xtls/xray-core v1.260327.0`）的源码已编译进 `sui`；不存在也不需要单独的 `xray` 可执行文件或 `SUI_XRAY_BINARY`。

| 客户端/核心 | Reality TCP Vision | XHTTP | 说明 |
| --- | --- | --- | --- |
| v2rayNG / v2rayN（选择 Xray-core） | 可使用 Xray 扩展 | 取决于客户端与所选 Xray-core 版本 | 导入标准 VLESS URI，并确认客户端实际使用 Xray-core。 |
| Mihomo **≥ 1.19.22** | 支持 Reality TCP Vision 配置 | 支持 XHTTP 配置 | 使用新版 Mihomo/Clash Meta 的订阅输出，并在自己的设备上验证。 |
| 旧 Clash / 旧 Clash Meta | 不保证 | 不保证 | 不作为兼容承诺。 |
| v2fly / V2Ray-core | 不支持 | 不支持 | Reality、XTLS Vision、XHTTP 是 Xray 扩展。 |

XHTTP 的支持范围比 TCP Vision 更窄。不能确认 `type=xhttp` 时，优先使用 TCP Vision 标准 URI。以上是核心/配置兼容性说明，**并不表示已经对每一种客户端版本和网络环境完成真实实测**。

## 7. 更新、恢复与敏感数据

升级时再次执行第 2 节的 ZIP 校验流程，解压后运行新的包内 `install-linux.sh`。不要用“下载一个裸 `sui` 并校验它”的方式替代发布流程。升级失败时先查看：

```bash
sudo journalctl -u s-ui -n 200 --no-pager
sudo systemctl status s-ui --no-pager
```

Reality 私钥、数据库和安装状态应视为敏感凭据。仅给 root 或受控服务账户最小权限，备份应加密并保存在受控位置；私钥泄露后应重新生成私钥、公钥与 short ID，并更新客户端订阅。

## 8. 官方参考

- s-ui-go 0.1 Release：<https://github.com/yongwei9527-art/s-ui-go/releases/tag/s-ui-go-v0.1.0>
- Xray-core `v26.3.27`：<https://github.com/XTLS/Xray-core/releases/tag/v26.3.27>
- Xray Reality/传输文档：<https://xtls.github.io/en/config/transport.html>
- Mihomo VLESS：<https://wiki.metacubex.one/en/config/proxies/vless/>
- Mihomo transport/XHTTP：<https://wiki.metacubex.one/en/config/proxies/transport/>
- Mihomo 官方发布记录（确认 `v1.19.22` 或更高）：<https://github.com/MetaCubeX/mihomo/releases>
- Xray-core MPL-2.0：<https://github.com/XTLS/Xray-core/blob/main/LICENSE>