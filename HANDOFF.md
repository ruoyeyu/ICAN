# ICAN 交接

采用上游 e13006fbaeabe14e4a6d27cba667ff1473009ebc；独立分支 security/ican-hardening。用户明确指定私有 ruoyeyu/ICAN 为项目仓库并授权上传，参考 fork 不修改。原审查源码目录保持不变。

已实现邀请码注册、持久可撤销会话、配置加密、统一 HTTP 出站检查、私有媒体与供应商链接、主动内容拒绝、后台功能隔离、去推广/默认渠道、透明的自定义视频参数、受影响依赖升级及非 root 镜像定义。额外发现并修复图片/音频任务客户端 ID 覆盖其他用户数据的问题。

先读 docs/security/supply-chain-review.md、docs/security/hardening-review.md、docs/security/deployment.md 和 evidence/，再处理 docs/progress/todo.md。上一轮 177 个 Go 测试及子测试、18 个前端测试、生产构建和 17 项 Go/Next 真实进程联通通过；最终最小镜像在 CI 完成非 root/只读/断网/重启恢复和漏洞扫描，高危/严重匹配为零，保留 23 中危、8 低危及 1 条不导入 openpgp 的模块匹配。详见 docs/security/runtime-review.md 和 evidence/image-ci.json。宿主机、原 HB 数据与实际画境接口仍有未完成门槛。没有付费模型调用或生产切换。

不得将“候选代码已上传”解释为“可以上线”。旧明文数据库需显式迁移；缺失的 HB 文件不能靠猜测补成已迁入。新增上传、容量、并发和重试等行为边界需先说明影响并取得用户确认。继续保留原站和独立备份，不能为赶上线降低门槛。

第二轮候选代码 `271689883f215dcd65120cbb7fa9e8d507c4056c`：移除固定 KIE 素材上传/Key 发送路径、远程供应商模型目录、提示词/版本订阅与未锁定 Agent 安装指令；后台保存/测试/转译统一拒绝已关闭协议。去除最终镜像 Sharp/libvips 原生解析器，增加 CSP、原生文件哈希与动态依赖盘点，构建依赖精确锁定。179 个 Go 测试及子测试、18 项前端测试、19 项真实进程检查通过。第二轮 CI、SBOM、原生清单与完整扫描独立记录为 evidence/second-pass-*；原历史证据保留。

31 条中低危匹配是 25 个去重 CVE，已按官方公告和触发条件逐项复核。zlib-85091 相应 Debian 源包核查支持版本范围误匹配，但保留官方标记分歧；glibc DNS search/LOCALDOMAIN 必须结合真实宿主机检查。没有将未见调用等同于证明安全，没有使用 ignore 或替换为不稳定系统库清零。

第二轮最终 [CI 40084004422](https://github.com/ruoyeyu/ICAN/actions/runs/40084004422) 全通过；镜像仍为 23 中危、8 低危、1 UNKNOWN，无高危/严重匹配。原生清单 285 个 ELF 均位于预期系统库/Node/Go 路径，Web native 与 set-ID 文件为零。Node 保留 aligned array-new 导入，两处直接调用定位到 V8 平台线程初始化，记录证据而非声明漏洞已修复。部署门槛仍未满足。
