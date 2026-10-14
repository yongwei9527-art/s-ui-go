# s-ui-go 前端

本目录是 [s-ui-go](../README.md) 的 Vue 前端，不是独立安装包。安装面板请使用根目录 README 中的发布包与校验命令；界面截图见 [screenshots.md](./screenshots.md)。本项目用于学习交流，部署前请自行完成安全评估。

## 本地开发

在本目录执行（CI 使用 Node.js `22.18.0`）：

```sh
npm ci
npm run dev
```

开发服务默认监听 `http://localhost:3000`，API 代理到本机 `http://localhost:2095/app/api`。需要同时运行后端；相关设置见本目录 `vite.config.mts`。

## 检查和构建

```sh
npm run lint
npm run test:unit
npm run build
```

`lint` 只检查，自动修复使用 `npm run lint:fix`。`build` 先运行 TypeScript 检查，再生成 `dist/`。

Go 主程序嵌入此构建目录，因此应先完成前端构建，再执行 Go 测试或打包；不要同时运行前端构建和 Go 的嵌入资源测试。构建发布包请使用根目录的统一脚本，不要只上传前端文件。

贡献说明见 [CONTRIBUTING.md](./CONTRIBUTING.md)；许可证见 [LICENSE](./LICENSE)。
