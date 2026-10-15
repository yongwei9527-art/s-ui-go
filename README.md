# s-ui-go

社区维护的代理管理面板，支持中文 / English，内置 sing-box 与 Xray-core。

**当前发布：[0.1.4](https://github.com/yongwei9527-art/s-ui-go/releases/tag/s-ui-go-v0.1.4)** · sing-box `v1.14.3` · Xray-core `v26.3.27` · [GPL-3.0](LICENSE)

## 下载

以下安装包仅适用于 **amd64 / x86_64**。Apple Silicon 和其他 ARM 设备请自行构建，不要安装这些包。

| 平台 | 安装包 | 校验文件 |
| --- | --- | --- |
| Windows | [ZIP](https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.4/s-ui-windows-amd64.zip) | [SHA256](https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.4/s-ui-windows-amd64.zip.sha256) |
| Linux（systemd） | [ZIP](https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.4/s-ui-linux-amd64.zip) | [SHA256](https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.4/s-ui-linux-amd64.zip.sha256) |
| macOS（Intel） | [ZIP](https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.4/s-ui-macos-amd64.zip) | [SHA256](https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.4/s-ui-macos-amd64.zip.sha256) |

先校验 ZIP，再解压、安装。旧 Release 保留；不要混用不同版本的 ZIP 和校验文件。

## Linux 安装与更新

适用于带 systemd 的 x86_64 Linux。需要 `curl`、`unzip`、`sha256sum` 及 root/sudo 权限。

**已有安装：先备份数据库和证书，再复制整个代码块执行。** 若原安装目录不是 `/usr/local/s-ui`，请修改最后一行，避免安装到错误目录。

```bash
(
  set -eu
  cd "$(mktemp -d)"
  base='https://github.com/yongwei9527-art/s-ui-go/releases/download/s-ui-go-v0.1.4'
  curl -fL --retry 3 -O "$base/s-ui-linux-amd64.zip"
  curl -fL --retry 3 -O "$base/s-ui-linux-amd64.zip.sha256"
  sha256sum --check s-ui-linux-amd64.zip.sha256
  unzip -q s-ui-linux-amd64.zip
  sudo bash ./install-linux.sh --lang zh --upgrade --install-dir /usr/local/s-ui
)
```

- **首次安装：** 去掉最后一行的 `--upgrade`，其余不变。
- **更新：** 保留已有管理员、面板及节点配置，不会自动将自定义节点改成新默认值。
- **旧证书位于 `/root/`：** 升级前迁移到服务用户可读的位置，并在面板修改路径。不要开放 `/root` 权限或使用 `chmod 777`。
- **检查状态：** `sudo systemctl status s-ui --no-pager`；排错日志：`sudo journalctl -u s-ui -n 50 --no-pager`。

更多说明见 [Ubuntu 部署与恢复](https://github.com/yongwei9527-art/s-ui-go/blob/latest-project-upload/docs/UBUNTU_DEPLOYMENT.md)。

## Windows / macOS

下载并校验对应 ZIP，解压后执行：

- **Windows：** 右键 `install-windows.bat` → 以管理员身份运行；使用 `s-ui-windows.bat` 管理服务。
- **Intel macOS：** 在解压目录运行 `sudo bash ./install-macos.sh`。

Windows/macOS 已有安装的操作与注意事项请先查看包内安装器提示及 [发布说明](RELEASE_NOTES.md)。

## 使用前必读

- 默认面板端口 `2095`、路径 `/app/`；订阅端口 `2096`、路径 `/sub/`。已有安装以原设置为准，HTTPS 取决于证书配置。
- 修改管理员凭据，限制管理端口来源；推荐通过 HTTPS 反代或 SSH 隧道访问，安装器不会保证云防火墙已放行。
- Reality 不需要自己的域名证书；普通 TLS 节点需匹配域名与有效证书。初始化自签证书不受信任，`insecure=true` 会跳过身份验证，不应作为生产解决方案。
- Hysteria2 / TUIC 等协议需要开放对应 **UDP** 端口。Xray 节点请使用支持相应扩展的客户端核心。
- 新安装的默认 Reality 端口为 `443/tcp`、Xray TCP Vision `10443/tcp`、XHTTP `2088/tcp`。非 443 Reality 端口有目标站封锁风险，启用前请阅读 [兼容性说明](https://github.com/yongwei9527-art/s-ui-go/blob/latest-project-upload/docs/DEVELOPMENT.md#linux-xhttp--reality-内置核心与兼容性)。
- 自动测试通过不等于所有平台实机、公网节点均已验收；未验证项见 [审计报告](https://github.com/yongwei9527-art/s-ui-go/blob/latest-project-upload/docs/AUDIT_REPORT.md)。

## 文档与开发

| 想做什么 | 文档 |
| --- | --- |
| 部署、升级、恢复 | [Ubuntu 指南](https://github.com/yongwei9527-art/s-ui-go/blob/latest-project-upload/docs/UBUNTU_DEPLOYMENT.md) · [Windows 指南](https://github.com/yongwei9527-art/s-ui-go/blob/latest-project-upload/docs/BACKEND_SETUP.md) |
| 配置证书与节点 | [配置与安全](https://github.com/yongwei9527-art/s-ui-go/blob/latest-project-upload/docs/CONFIGURATION.md) · [协议指南](PROTOCOL_GUIDE.md) |
| 构建源码、确认客户端兼容性 | [开发与兼容性](https://github.com/yongwei9527-art/s-ui-go/blob/latest-project-upload/docs/DEVELOPMENT.md) |
| 查看改动与已知限制 | [发布说明](RELEASE_NOTES.md) · [审计报告](https://github.com/yongwei9527-art/s-ui-go/blob/latest-project-upload/docs/AUDIT_REPORT.md) |
| 了解目录、参与开发 | [目录导航](https://github.com/yongwei9527-art/s-ui-go/blob/latest-project-upload/docs/README.md) · [贡献指南](https://github.com/yongwei9527-art/s-ui-go/blob/latest-project-upload/CONTRIBUTING.md) |

## 致谢与许可

感谢 S-UI 上游作者、社区贡献者，以及 sing-box、Xray-core、Go、Vue 等开源项目。
本项目是非官方社区维护版本，按 [GPL-3.0](LICENSE) 提供，不作担保；再分发还须遵守各依赖许可，见 [Xray-core 声明](XRAY-CORE-NOTICE.md)。
