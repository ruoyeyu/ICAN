# ICAN 交接

当前状态：供应商模板和可配置 Docker 资源已提交；源码 53f8aa31d8fd803668cc138b3d91262ea9903c89 / CI 40184337184 全通过，GHCR 固定 digest 可用。v0.1.0 Release 尚未创建：GitHub 拒绝 Actions 令牌的标签移动，原子操作未改标签。一次性重置脚本已移除，正常发布门槛保留。最后需要仓库所有者按 docs/security/version-reset.md 重置两个精确标签，再手动运行 Publish checked pre-release。未改服务器。
最新源码 `53f8aa31d8fd803668cc138b3d91262ea9903c89` 的 CI 40184337184 已通过：Go/Web 31/导演台 294、隔离与账号/模板重启恢复、两组 Compose 限额、运行时回归和扫描均通过。固定 digest 与匿名核验见 `docs/security/evidence/ghcr-v0.1.0-restart-publication.json`（pending-test 相对路径为 `../security/evidence/ghcr-v0.1.0-restart-publication.json`）。发布复核通过，但标签重置权限阻断尚未解除；没有操作服务器。
本轮追加：站点供应商模板目录与管理员增删改、现有选择器动态读取；个人导入保留模板并沿用账号同步。公共仓库不预置私人供应商。模板保存为独立 JSON，个人渠道保留快照，删除不改已有渠道/任务。使用及兼容说明见 docs/security/supplier-templates.zh-CN.md。后端/Web 与生产浏览器验证已通过，见 docs/security/evidence/supplier-template-local.json；新镜像发布验收进行中，未操作服务器。

当前任务：用户已清空 Release，并明确授权把最新完整实现重新编号为 v0.1.0，清理旧版本标签，之后从新基线递增。版本重置同时将 Docker CPU/内存改为部署者可配置，`.env.example` 与 Compose 顶部提供入口，检查器按显式期望值校验。保留所有应用功能、安全修复、数据库/配置格式、数据和密钥。新版本需重新通过 CI 和 GHCR 发布，服务器不改动。下方版本号及证据为重置前历史，不是当前下载指引。

本轮目标 v0.1.3：沿用原渠道/Goja 转译，增加表单与通用预设、v1 无凭据导入导出、逐模型参数和参考编码。视频明确 URL 输入复用上传及归属授权，补齐显式内容下载与跨来源重定向拒绝；私有供应商配置不进公开仓库。没有修改生产服务器。配置版本与回滚要求见 docs/security/custom-api-templates.md；本地 Go/Web/导演台/生产构建、浏览器导入编辑导出与请求捕获、v0.1.2 数据副本升级/回滚已验证，见 docs/security/evidence/api-config-ui-local.json。CI 40180373140 构建、检查与 GHCR 发布通过；源码 4cef8067138572047233b72e0fe67373bc1d3242，digest 见 docs/security/evidence/ghcr-v0.1.3-publication.json；匿名 manifest/config、版本标签已核对。Release 40180754949 发布成功；v0.1.3 tag 指向清理后的源码，六个附件已下载并核对 SHA256，见 docs/security/evidence/prerelease-v0.1.3.json。主线已移除旧记录的内部标识，历史提交未重写。

v0.1.2 本轮修改：修复后台 local 存储误转 s3，保留渠道权限与配置；扩展现有图片模板的参考编码、二进制 multipart、原始参数、显式同源内容下载；统一 ICAN/画布文案，删去登录页标题和说明。未运行外部插件，专用供应商配置不入公共仓库。见 docs/security/custom-api-templates.md。Go 全套、Web 26 项、导演台 294 项、生产前端构建、真实浏览器登录/双账号/导演台及后台保存回归通过；85 组专用模板请求 fixture 通过，证据为 docs/security/evidence/custom-api-local.json。CI 40110294471 构建、测试、扫描、实际 Compose 与发布全通过；镜像源码 9b3b8db415e785c7e179e135185f257a016d697b，digest 见 docs/security/evidence/ghcr-v0.1.2-publication.json。已核对匿名 manifest、配置 digest、源码及版本标签。Release 40110729630 发布成功，tag 与六个附件 SHA256 已核对，见 docs/security/evidence/prerelease-v0.1.2.json，未操作服务器。

按用户要求删除 v0.1.0 GitHub Release 及其附件，一次性任务 40105399560 成功，随后移除该任务定义。v0.1.1 Release/附件、源码 tag 和 GHCR 历史镜像保留；README 旧下载入口已移除。证据见 docs/security/evidence/removed-release-v0.1.0.json。未改应用或服务器，生产待办无需调整。

v0.1.1 公共镜像已完成重新编译与发布，CI 40103981848 全通过；源码 8b5d9aeb06d5ea1ad967f36b31fdfe2798efc0a0，digest 及匿名拉取验证见 docs/security/evidence/ghcr-v0.1.1-publication.json。Release 使用新版本源码及同一份已验证镜像，保留 v0.1.0 tag/附件；Release 40104375905 发布成功，源码 tag、公开预发布状态与六个附件 SHA256 已核对，证据为 docs/security/evidence/prerelease-v0.1.1.json。没有服务器部署。

用户要求重新编译并提供可共同使用的公共版本，本轮发布目标为 v0.1.1。版本号、Web/Director 包元数据和 OCI 标签同步更新；主线流水线重新构建并验证镜像后才发布 Release。无需生产服务器编译，不操作线上部署。

本轮修复游客画布入口和无归属浏览器缓存自动同步：服务端页面鉴权、根路径会话 Cookie、登录后使用与账号缓存隔离。详见 docs/security/login-required.md。保留旧本地缓存、数据库、媒体和密钥；原 v0.1.0 镜像不变，需采用本次成功 CI 的新 digest。未操作生产服务器。

用户要求删除泛化安全免责声明；公开说明按具体措施、测试结果与待办表达。真实上线阻断项和原始检查证据继续保留。

v0.1.0 预发布已发布，流水线 40099763230 全通过；已核对 tag 指向镜像源码、预发布状态和六个附件 SHA256。证据见 `docs/security/evidence/prerelease-v0.1.0.json`。用户授权增加 v0.1.0 预发布入口。`.github/workflows/release.yml` 复核已发布镜像的源码、成功流水线和 digest 后发布；只使用 GitHub 临时令牌，不要求网页登录。源码 tag 指向镜像对应的 `4702d09c590769f5ec73ef3de6b312702bcea7e3`，说明和附件不代表生产验收通过。

本轮用户授权 GitHub 构建、检查后发布 GHCR，禁止自动部署服务器。Compose 固定 1.6 CPU / 1.5 GiB；服务器仅拉 digest，更新保留原数据和密钥。流程见 `docs/security/ghcr-deployment.md`，现有生产阻断项继续生效。首次 GHCR 流水线 40098999311 全通过，源码 `4702d09c590769f5ec73ef3de6b312702bcea7e3`；实际 digest 和检查证据见 `docs/security/evidence/ghcr-publication.json`。

早期 0.1.2 阶段记录（仅追溯）：只维护 `main`。最新工作顺序：暂停功能扩充，先收尾资源边界、会话凭据和更新恢复，再做最终镜像与服务器部署验收；详见 `docs/progress/todo.md`。本轮精简记录见 `docs/security/repository-cleanup.md`。保留数据库模型、持久数据路径、密钥和共享协议辅助函数；没有修改线上部署。下面的旧候选分支记录仅用于追溯历史验证，不是当前检出指令。

用户最新要求：确认不影响当前构建的扫描项不计入有效风险数；原始证据保留。runtime-candidate-ci.json 为 GO-2026-5932 增加按具体镜像绑定的排除及有效计数，旧待复核 CVE 不随之自动排除。用户还明确要求频繁更新容器时保留既有账号、媒体、个人 API 和管理员中转站配置；AGENTS.md 已改为要求版本化增量迁移。现有绑定目录和加密配置支持持久保存，但当前启动 AutoMigrate 不等于完整迁移机制，跨版本恢复/迁移仍列入阻断项。详见 docker-operations.md 数据保留章节。

用户已确认采用 Docker。历史分支（已合入 main）增加不可变镜像部署、显式环境变量、只读容器策略检查、实际 Compose 与原生运行回归；同时试验签名验证后的 Wolfi glibc-dynamic 基础层。参见 docs/security/docker-operations.md。这个候选不自动升级为生产，原 Debian 的中低危账目仍保留；源补丁/公告覆盖、服务器隔离和功能/迁移门槛未完成前保留现站。

Docker 候选代码 dcbd8399213cf290994fcd69fa224697535d280c 的 CI 40086299774 全通过。实际 Compose、非 root/只读/seccomp、账号/私有媒体/重启、原生分配及封闭 DNS/TLS、SBOM/扫描均已验证；23 个 ELF（原 285），Node/Go 二进制与原基线一致。扫描 0 严重/高/中/低、1 UNKNOWN；没有自动关闭原 25 个 CVE。完整证据为 evidence/runtime-candidate-*，没有发布镜像或切换服务器。首次 fixture 初始化失败及其最小权限修正也保留记录。

采用上游 e13006fbaeabe14e4a6d27cba667ff1473009ebc；独立分支 security/ican-hardening。用户明确指定私有 ruoyeyu/ICAN 为项目仓库并授权上传，参考 fork 不修改。原审查源码目录保持不变。

已实现邀请码注册、持久可撤销会话、配置加密、统一 HTTP 出站检查、私有媒体与供应商链接、主动内容拒绝、后台功能隔离、去推广/默认渠道、透明的自定义视频参数、受影响依赖升级及非 root 镜像定义。额外发现并修复图片/音频任务客户端 ID 覆盖其他用户数据的问题。

先读 docs/security/supply-chain-review.md、docs/security/hardening-review.md、docs/security/deployment.md 和 evidence/，再处理 docs/progress/todo.md。上一轮 177 个 Go 测试及子测试、18 个前端测试、生产构建和 17 项 Go/Next 真实进程联通通过；最终最小镜像在 CI 完成非 root/只读/断网/重启恢复和漏洞扫描，高危/严重匹配为零，保留 23 中危、8 低危及 1 条不导入 openpgp 的模块匹配。详见 docs/security/runtime-review.md 和 evidence/image-ci.json。宿主机、既有部署数据与实际外部 API 接口仍有未完成门槛。没有付费模型调用或生产切换。

不得将“候选代码已上传”解释为“可以上线”。旧明文数据库需显式迁移；缺失的旧应用文件不能靠猜测补成已迁入。新增上传、容量、并发和重试等行为边界需先说明影响并取得用户确认。继续保留原站和独立备份，不能为赶上线降低门槛。

第二轮候选代码 `271689883f215dcd65120cbb7fa9e8d507c4056c`：移除固定 KIE 素材上传/Key 发送路径、远程供应商模型目录、提示词/版本订阅与未锁定 Agent 安装指令；后台保存/测试/转译统一拒绝已关闭协议。去除最终镜像 Sharp/libvips 原生解析器，增加 CSP、原生文件哈希与动态依赖盘点，构建依赖精确锁定。179 个 Go 测试及子测试、18 项前端测试、19 项真实进程检查通过。第二轮 CI、SBOM、原生清单与完整扫描独立记录为 evidence/second-pass-*；原历史证据保留。

31 条中低危匹配是 25 个去重 CVE，已按官方公告和触发条件逐项复核。zlib-85091 相应 Debian 源包核查支持版本范围误匹配，但保留官方标记分歧；glibc DNS search/LOCALDOMAIN 必须结合真实宿主机检查。没有将未见调用等同于证明安全，没有使用 ignore 或替换为不稳定系统库清零。

第二轮最终 [CI 40084004422](https://github.com/ruoyeyu/ICAN/actions/runs/40084004422) 全通过；镜像仍为 23 中危、8 低危、1 UNKNOWN，无高危/严重匹配。原生清单 285 个 ELF 均位于预期系统库/Node/Go 路径，Web native 与 set-ID 文件为零。Node 保留 aligned array-new 导入，两处直接调用定位到 V8 平台线程初始化，记录证据而非声明漏洞已修复。部署门槛仍未满足。


## Director candidate continuation

See docs/security/director-integration.md and director/README.md. Director is built from pinned MIT source with opaque iframe isolation. No ComfyUI/RunningHub backend enablement, schema migration or production deployment. Final image and account/cross-device acceptance remain separate gates.

Director image CI 40093383952 passed on fbc1b4bea836566f5422ba104867984de9d64f6d. See docs/security/director-verification.json for image/artifact hashes and raw scan counts. Production host, authenticated cross-device checks and legacy scene migration remain unverified.
