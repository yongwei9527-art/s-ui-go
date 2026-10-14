# 前端贡献指南

本目录随 s-ui-go 主仓库维护。先阅读根目录 [贡献说明](../CONTRIBUTING.md) 和本目录 [开发说明](./README.md)，不要使用旧的独立前端仓库占位地址。

## 开发流程

1. Fork s-ui-go 仓库，克隆自己的 Fork，并从当前开发分支创建修复分支。
2. 进入 `frontend/`，运行 `npm ci` 安装锁定依赖。
3. 使用 `npm run dev` 开发；默认端口 `3000`，需配合本地后端的 `2095` 端口。
4. 修改代码并为修复添加 `src/**/*.test.ts` 回归测试。
5. 依次运行 `npm run lint`、`npm run test:unit`、`npm run build`。
6. 涉及后端或打包时，等前端构建结束后再运行 Go 检查，最后提交 Pull Request，说明原因、验证结果和未验证项。

## 目录

- `src/components/`：协议、传输、TLS 等共用组件。
- `src/layouts/`：导航布局与弹窗。
- `src/views/`：页面。
- `src/locales/`：多语言翻译。
- `src/plugins/`：HTTP 等插件与工具。
- `src/store/`：Pinia 状态。
- `src/types/`：TypeScript 数据结构及默认配置。
- `src/styles/`：全局样式。

## 提交注意

- 遵循现有 Vue/TypeScript 风格；`npm run lint` 不会自动改文件，自动修复另用 `npm run lint:fix`。
- 不提交 `node_modules/`、`dist/`、数据库、令牌、证书私钥或真实用户订阅。
- 不仅测试成功路径：也应覆盖空值、失败响应和异步请求乱序。
- UI 文字变化请检查相关翻译，协议变化请检查订阅导出及客户端兼容性。
- 安装包发布由主仓库统一构建，不能用仅更新源码代替更新 Release 资产。

报告问题请提供最小复现、操作系统、浏览器/客户端版本及脱敏日志，可参考本目录 `.github/ISSUE_TEMPLATE/` 模板。请尊重他人，并勿将本项目用于违法用途。
