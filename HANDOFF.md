# ICAN 交接

采用上游 e13006fbaeabe14e4a6d27cba667ff1473009ebc；独立分支 security/ican-hardening。用户明确指定私有 ruoyeyu/ICAN 为项目仓库并授权上传，参考 fork 不修改。原审查源码目录保持不变。

已实现邀请码注册、持久可撤销会话、配置加密、统一 HTTP 出站检查、私有媒体与供应商链接、主动内容拒绝、后台功能隔离、去推广/默认渠道、透明的自定义视频参数、受影响依赖升级及非 root 镜像定义。额外发现并修复图片/音频任务客户端 ID 覆盖其他用户数据的问题。

先读 docs/security/hardening-review.md、docs/security/deployment.md 和 evidence/，再处理 docs/progress/todo.md。本轮 177 个 Go 测试及子测试、18 个前端测试、生产构建和 17 项 Go/Next 真实进程联通通过；最终最小镜像在 CI 完成非 root/只读/断网/重启恢复和漏洞扫描，高危/严重匹配为零，保留 23 中危、8 低危及 1 条不导入 openpgp 的模块匹配。详见 docs/security/runtime-review.md 和 evidence/image-ci.json。宿主机、原 HB 数据与实际画境接口仍有未完成门槛。没有付费模型调用或生产切换。

不得将“候选代码已上传”解释为“可以上线”。旧明文数据库需显式迁移；缺失的 HB 文件不能靠猜测补成已迁入。新增上传、容量、并发和重试等行为边界需先说明影响并取得用户确认。继续保留原站和独立备份，不能为赶上线降低门槛。
