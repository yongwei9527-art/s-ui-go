# 文档与目录导航

[返回项目首页](../README.md)

## 文档

- [Ubuntu 部署与恢复](UBUNTU_DEPLOYMENT.md)
- [Windows 后端搭建](BACKEND_SETUP.md)
- [节点、证书与安全配置](CONFIGURATION.md)
- [源码构建与客户端兼容性](DEVELOPMENT.md)
- [审计报告与未验证项](AUDIT_REPORT.md)
- [协议指南](../PROTOCOL_GUIDE.md) · [发布说明](../RELEASE_NOTES.md)

## 为什么保留这些根目录文件

| 文件或目录 | 作用 |
| --- | --- |
| `api/`、`app/`、`core/`、`database/`、`service/` 等 | Go 包；搬动会改变导入路径，不为外观随意改名或移动。 |
| `frontend/` | Web 面板源码。 |
| `internal/`、`tests/` | 内部实现和回归测试。 |
| `install*`、`uninstall*`、`s-ui*` | 用户与安装包的稳定安装、卸载、管理入口。 |
| `build*`、`update-xray-source.ps1` | 本地和 CI 构建维护入口。 |
| `Dockerfile`、`docker-compose.yml`、`entrypoint.sh` | Docker 构建与启动入口。 |
| `go.mod`、`go.sum`、`main.go` | Go 项目入口与依赖定义。 |
| `PROTOCOL_GUIDE.md`、`RELEASE_NOTES.md`、`XRAY-CORE-NOTICE.md`、`LICENSE` | 安装包需要附带的文档与许可，保持现有路径。 |
| `docs/` | 部署、开发、配置及审计说明。 |
| `.github/` | GitHub 工作流与问题模板。 |

本次仅整理文档；安装路径、脚本名称、默认节点、数据库和已发布资产均未改变。
安装包内的 README 保留发布时内容，仓库文档后续优化不覆盖旧资产。
