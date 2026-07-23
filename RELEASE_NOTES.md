# s-ui-go 0.1 发布说明

> **用户可见名称：** s-ui-go 0.1
> **发布标签：** [`s-ui-go-v0.1.0`](https://github.com/yongwei9527-art/s-ui-go/releases/tag/s-ui-go-v0.1.0)

## 本次发布的交付与校验

Linux 发布资产是一对不可分割的文件：

- `s-ui-linux-amd64.zip`
- `s-ui-linux-amd64.zip.sha256`

正确流程是：**校验 ZIP → 解压 ZIP → 使用其中的 `sui` 或 `install-linux.sh`**。校验文件校验的是 ZIP 本身；本次发布不要求、也不提供作为安装契约的裸二进制校验文件。不要使用任何可变分支脚本或 `curl | bash` 作为 bootstrap 信任来源。

```bash
sha256sum --check s-ui-linux-amd64.zip.sha256
unzip -t s-ui-linux-amd64.zip
unzip -q s-ui-linux-amd64.zip -d package
cd package && sudo bash ./install-linux.sh
```

Ubuntu 包内安装需要 systemd，默认目录为 `/usr/local/s-ui`。全新安装会严格检查 Reality 目标、端口与双核心实际启动；任一环节失败会中止并回滚本次变更。已有安装升级会保留现有面板、管理员和协议配置，并在失败时恢复安装前状态。

## 双核心与内置 Xray-core

- sing-box 与 Xray-core 同进程协作运行；默认端口为 sing-box Reality `443/tcp`、Xray Reality TCP Vision `10443/tcp`、Xray Reality XHTTP `2088/tcp`。
- Xray-core 的 **`v26.3.27`** 源码（Go 模块 **`github.com/xtls/xray-core v1.260327.0`**）已编译进 `sui`。
- Linux ZIP 不分发、不启动独立 `xray` 文件，也不需要 `SUI_XRAY_BINARY`。
- Xray-core 是 MPL-2.0 许可的上游项目；详见 [XRAY-CORE-NOTICE.md](XRAY-CORE-NOTICE.md)。

[Xray 官方 Reality/传输文档](https://xtls.github.io/en/config/transport.html) 说明非 `443` Reality 端口可能带来 Reality 目标的封锁风险。`10443` 与 `2088` 因此应在部署前评估：使用不同 IP、进行端口映射，或明确接受风险；不能接受时不要启用这两个默认非 443 入站。

## 客户端兼容性边界

| 客户端/核心 | Reality TCP Vision | XHTTP | 结论 |
| --- | --- | --- | --- |
| v2rayNG / v2rayN（Xray-core） | 支持所需 Xray 扩展 | 支持取决于客户端与 Xray-core 版本 | 必须选择 Xray-core。 |
| Mihomo **≥ 1.19.22** | 支持 Reality TCP Vision 配置 | 支持 XHTTP 配置 | 使用新版 Mihomo/Clash Meta。 |
| 旧 Clash / 旧 Clash Meta | 不保证 | 不保证 | 不作为兼容目标。 |
| v2fly / V2Ray-core | 不支持 | 不支持 | 不能用于 Reality、XTLS Vision、XHTTP。 |

XHTTP 的实际支持范围比 TCP Vision 窄；无法确认 `type=xhttp` 时应优先使用 TCP Vision 的标准 URI。以上是版本/核心能力边界，**不是对所有客户端版本已经进行真实端到端实测的声明**。

## 网络与管理面边界

安装成功并不等于公网端口已放通。云安全组、云防火墙和网络 ACL 仍须由部署者确认 `443/tcp`、`10443/tcp`、`2088/tcp`。安装器不会承诺自动开放面板/订阅端口 `2095/2096`；生产环境应优先将其限制为本机，再经 HTTPS 反代或 SSH 隧道访问。

## Official references

- Release: <https://github.com/yongwei9527-art/s-ui-go/releases/tag/s-ui-go-v0.1.0>
- Xray-core v26.3.27: <https://github.com/XTLS/Xray-core/releases/tag/v26.3.27>
- Xray transport / Reality: <https://xtls.github.io/en/config/transport.html>
- Mihomo VLESS: <https://wiki.metacubex.one/en/config/proxies/vless/>
- Mihomo transport: <https://wiki.metacubex.one/en/config/proxies/transport/>
- Mihomo releases (v1.19.22+): <https://github.com/MetaCubeX/mihomo/releases>

## 历史发布记录（保持历史事实）

### v1.0.0（历史）

以下记录仅用于保留当时的发布事实，不表示当前的版本命名或发布标签。

- 提供 Windows、Linux、macOS 的 amd64 ZIP 发布包。
- Web 面板语言整理为 `中文` / `English`，默认语言跟随浏览器语言。
- 发布包包含 `RELEASE_NOTES.md` 与 `PROTOCOL_GUIDE.md`，便于离线查看说明。
- 补充 TLS / ECH / WebSocket / TUIC / Hysteria2 等订阅转换与校验说明。

## 致谢

本项目基于 S-UI / s-ui-go 的开源成果继续整理和改进。感谢原作者、社区贡献者以及 Xray-core、sing-box、Go、Vue、Vite、TypeScript 等上游项目与维护者。