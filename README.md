# ICAN

基于 Infinite Canvas 的个人 API 画布工作台：无限画布，图片/视频/音频生成，账号配置与项目同步，服务器统一素材存储。

本分支为安全加固候选版本，**未通过生产部署验收，不应直接替换现站**。不附带供应商、付费模型、推广链接或一键安装脚本。注册需要管理员签发的一次性邀请码。

- [实施与验证报告](docs/security/hardening-review.md)
- [运行组件与镜像审核](docs/security/runtime-review.md)
- [部署门槛、备份和回滚](docs/security/deployment.md)
- [Docker 运行、验收与更新](docs/security/docker-operations.md)
- [交接记录](HANDOFF.md)
- [文档索引](docs/index.md)

开发验证：Go 1.27.3 执行 `go test ./...`；Node 24.22.1、Bun 1.4.4 在 `web` 执行 `bun install --frozen-lockfile --ignore-scripts`、`bun test src`、`bun run build`。检查 `.env.example` 后自行配置独立密钥；不包含可用的默认管理员密码。

原项目及贡献者版权保留，采用 AGPL-3.0；见 [LICENSE](LICENSE) 与 [NOTICE](NOTICE)。依赖许可证保留于相应组件中，静态引用的 shadcn 样式许可证位于 `LICENSES/`。
