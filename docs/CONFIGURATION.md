# 节点、证书与安全配置

[返回项目首页](../README.md)

## Xray 可选默认模板

页面提供 Reality TCP Vision（10443/tcp）、XHTTP auto（2088/tcp）、XHTTP stream-one（2089/tcp）三个模板；选模板不自动创建或启用节点。新节点保存时生成独立密钥，确认端口空闲、目标可达并分配客户端后再启用。已有配置不覆盖，Reality 无需自己的域名证书。

## 默认节点 TLS 与升级

默认入站包含 VLESS Reality、Hysteria2、TUIC、Trojan、VMess WS、Shadowsocks 2022、Naive、AnyTLS 和 Hysteria。Reality 与 Shadowsocks 不使用普通 TLS 域名证书。

新安装无有效域名证书时，默认 TLS 使用保留名称 `bootstrap.s-ui.invalid`（无需解析），每个模板生成独立私钥和自签证书；VMess WebSocket Host 使用部署地址，不再冒用第三方域名。其他默认 TLS 模板未提供域名证书时仍是**初始化专用自签证书**，不等于受信任证书；初始化订阅明确设置 `insecure=true`，Clash 对应 `skip-cert-verify=true`。这会失去服务器身份认证，仅用于初始化测试，不建议生产使用。已有节点不自动重建或改写。客户端必须支持导出的对应验证设置。生产部署推荐使用自己的域名及完整证书链，保持证书验证开启。UDP 节点还必须放行对应 UDP 端口，更新程序不会自动解决云防火墙或客户端不支持的问题。

可以显式设置以下三个环境变量，让程序初始化或迁移未修改的默认 TLS 模板：

```text
S_UI_DEFAULT_TLS_SERVER_NAME=你的证书域名
S_UI_DEFAULT_TLS_CERTIFICATE_PATH=完整证书链的绝对路径
S_UI_DEFAULT_TLS_KEY_PATH=对应私钥的绝对路径
```

三项必须完整提供；已有安装还须显式设置 `S_UI_DEFAULT_TLS_MIGRATE_BOOTSTRAP=true` 才执行迁移；证书须与私钥匹配、未过期且包含该域名。已有自定义 TLS 配置不会被覆盖，入站 ID、端口和用户保留。迁移完成后需重新更新客户端订阅。不要把自己的私钥提交到 GitHub。

systemd 不会自动继承终端环境变量，须在 `sudo systemctl edit s-ui` 中添加 `[Service]` 下的 `Environment="变量名=值"`，再执行 `sudo systemctl daemon-reload && sudo systemctl restart s-ui`。证书及私钥须位于服务用户可读取、systemd 沙箱可访问的位置（推荐安装目录的 `cert/`）；不要开放 `/root` 权限，也不要使用 `chmod 777`。本版本的 Linux 管理脚本已将新签发证书目录改为安装目录下的 `cert/`，使用受限目录权限和服务组读取权限；旧 `/root` 证书不会自动迁移，升级前需在面板更新对应证书路径。本版本新增：经上述环境变量配置的有效证书将保存为外部 `certificate_path` / `key_path` 引用，不再复制私钥进数据库；核心支持文件变化重载。续期必须更新这两个路径对应的文件，并保持服务用户可读；推荐部署钩子在证书和私钥都更新完成后重启 `s-ui`，避免两文件更新中间态。`s-ui-go-v0.1.2` 及以前已导入数据库的内联证书仍需在面板改为外部路径，不能仅靠更新磁盘文件。迁移成功后可移除一次性环境变量。

## 安全提醒

### 反向代理信任边界

仅在面板位于实际受控的反向代理之后时，才设置 `S_UI_TRUSTED_PROXIES`。该环境变量接受以逗号分隔的代理 IP 或 CIDR，例如：

```bash
S_UI_TRUSTED_PROXIES=127.0.0.1,10.0.0.0/24
```

- 只填写真实反向代理的地址范围；反向代理必须安全地覆写或追加 `X-Forwarded-For` 与 `X-Forwarded-Proto`。
- 不要配置 `0.0.0.0/0` 或 `::/0`，也不要在信任宽泛网段时将面板监听端口直接暴露到公网。
- 未配置该变量时，来自客户端的伪造转发头不会影响登录限流、审计 IP 或 Secure Cookie 判定。

生产环境使用前请务必：

- 修改默认管理员账号和密码
- 修改默认端口和访问路径
- 限制面板访问来源或增加反向代理鉴权
- 妥善保管数据库、证书和配置文件
- 定期检查 Release 更新和依赖安全风险

请勿提交以下本机数据：

- `frontend/node_modules/`
- `frontend/dist/`
- `db/`、`logs/`、`cert/`
- 本地二进制、临时包和备份文件
- `.env`、`.vscode/`、`.claude/` 等本地配置
