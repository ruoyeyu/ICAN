> Current pull-only GHCR procedure: [ghcr-deployment.md](ghcr-deployment.md). CPU is fixed at 1.6 and memory at 1.5 GiB. Historical CI evidence below remains unchanged.

# Docker 运行与维护

用户已选择 Docker。采用单个应用容器（Next + Go）、SQLite 和本地私有媒体，沿用已有功能；不新增数据库或消息队列服务。推荐放在专用虚拟机/云服务器，避免与其他重要业务共用宿主机权限和数据。当前只是待验收交付，不表示已部署到 现有生产站点。

## 镜像与运行边界

- Dockerfile 多阶段构建：只把运行产物复制到最终无 shell、无包管理器镜像，UID/GID 10001；Go/Node/Bun、运行基础镜像、Actions 均固定版本及摘要。
- docker-compose.yml 只运行 `ICAN_IMAGE` 指定且已经加载的镜像，不在部署时构建、拉取浮动标签或自动更新。使用完整 `仓库@sha256:...`，或者经过验证的本地 `sha256:...` Image ID；两种摘要含义不同，不能互换。`scripts/check-container.py` 拒绝浮动标签。
- 当前验证平台为 linux/amd64，Compose 明确固定该平台。运行根目录只读，唯一持久可写挂载为 /app/data；/tmp 和 Next cache 使用限额 tmpfs。数据目录必须预先存在，Compose 不会自动创建权限错误的宿主机目录。
- 仅向宿主机 127.0.0.1 发布前端 3000，8080 不对宿主机发布。专用 bridge 网络本身不阻止出站，必须另验宿主机出站 ACL。
- 丢弃全部 capabilities、禁止提权，保留默认 seccomp，不允许 privileged、host namespace、额外设备或 Docker socket。优先评估 Rootless Docker；本次 CI 是标准 Linux Docker，不能据此宣称 Rootless 已验收。
- Compose 显式传入应用必需的环境变量，数据库、媒体和日志固定在 /app/data 下。不再把整份 .env（例如意外的 NODE_OPTIONS、代理或动态库变量）注入应用。
- CPU 1.6 核、内存 1.5 GiB 已由用户指定并固定在 Compose；其余资源、认证速率、素材链接期限保持必填。历史 CI fixture 的 1 GiB、2 CPU、256 PID 等不作为生产建议；数据和日志磁盘配额、上传/并发等应用边界仍待确认。

## 候选运行库与漏洞账目

已合入 `main` 的运行镜像方案采用 `cgr.dev/chainguard/glibc-dynamic@sha256:2827063c9efb4ba971d8621eb75f4e61b8d19850a6293a797df37358cc6f687b`。公开 Starter 为 Wolfi 开源软件包，无 shell/包管理器；仅 C.UTF-8 locale。它与原 Debian 基础镜像是不同发行版，变更须单独验收，原已验证版本仍保留。

基础镜像签名已按发布者 GitHub Actions 身份和 OIDC issuer 验证，见 evidence/runtime-candidate-base-signature.log；扫描原始证据独立保存。基础层七个包包含 glibc 2.44-r9、GCC runtime 16.2.0-r1，当前该基础层没有扫描匹配，但不等于最终应用镜像无漏洞，也不等于原来的 25 个 CVE 全部修复。Wolfi 公告覆盖与 Debian 不同，保留 supply-chain-review.md 中的原始记录和未完成事项。

候选须核对 GCC/glibc 实际源版本与公告、x86-64 CPU 指令集需求、locale、TLS/DNS 和原生运行兼容性。CI 增加有界 aligned allocation 测试、长 LOCALDOMAIN 配置解析、封闭网络中的 DNS/TLS 正反例。这些只覆盖所写触发条件，不是完整 CVE 复现或间接调用证明。源码/公告覆盖和 locale 限制见 evidence/runtime-candidate-source-review.json。

## 实际验证结果

[CI 40086299774](https://github.com/ruoyeyu/ICAN/actions/runs/40086299774) 全通过，代码 `dcbd8399213cf290994fcd69fa224697535d280c`，最终 Image ID `sha256:33f6705156851ae767376806ddd56453557ef7c03705062c11ebc77cc10521ae`；没有发布 registry 镜像或部署现站。证据汇总在 [runtime-candidate-ci.json](evidence/runtime-candidate-ci.json)，完整日志、组件清单、扫描和 SBOM 在同目录。

- Go 测试、18 项前端测试与生产构建通过；5 个策略检查测试含多组危险配置反例。
- 实际镜像通过非 root、只读根目录、无 capabilities、NoNewPrivs、有效 seccomp、邀请码注册、私有媒体/授权撤销、退出及账号重启恢复检查。
- 实际 Compose 文件通过 create 后的配置检查、启动后应用测试与宿主机回环端口访问；全程只有一次性 fixture 数据与凭据。
- aligned scalar/array allocation、长 DNS search 配置解析及封闭网络 DNS/TLS 正反例通过。仍不代替生产 DNS/证书环境验收。
- 最终镜像 7 个 OS 包、13 个 Node 包、49 个 Go 模块；23 个 ELF，原基线为 285 个。系统 zlib/OpenSSL、gconv 模块不再进入该候选镜像；Node 自带静态库仍存在并单独记录。Node 与 Go 主程序 SHA256 均与原已验证版本一致。无 Web 原生文件和 set-ID 文件。
- 当前完整扫描为 0 CRITICAL、0 HIGH、0 MEDIUM、0 LOW、1 UNKNOWN；UNKNOWN 仍是未导入 openpgp 的 x/crypto 模块匹配。原 Debian 的 25 个 CVE **没有因扫描结果变化自动关闭**，需结合真实移除的文件、更新的源码和发行版覆盖逐项确认。

首次 CI 的 Compose fixture 初始化因 CAP_CHOWN 不授予 chmod 权限而失败；现已由宿主机目录所有者先设置模式，再在受限一次性容器内 chown。没有向应用添加 capabilities。失败及修正证据保留为 runtime-candidate-first-ci.*。

## 容器更新与业务数据保留

当前 Compose 使用 SQLite；业务数据通过 `ICAN_DATA_DIR` 绑定到容器 `/app/data`。镜像包含应用代码，持久目录在镜像之外。正常删除/重建应用容器不会删除这个宿主机目录；必须确保新容器挂载的确实是原目录，而不是一个新的空目录。

| 数据 | 当前保存位置 | 升级要求 |
|---|---|---|
| 用户、密码哈希、会话、邀请码、画布、任务/记录及素材索引 | `/app/data/ican.db`；运行期间可能存在 SQLite WAL/SHM | 保留数据库，结构变化使用已验证的增量迁移 |
| 图片、视频、音频本体（服务器本地存储） | `/app/data/media` | 与数据库索引一致备份，保留目录与文件权限 |
| 登录用户已同步的个人 API 地址/Key、模型和自定义配置 | 数据库 `user_configs`，敏感配置 AES-GCM 加密 | 保留账号归属与原加密密钥；浏览器未同步的配置不在服务器备份中 |
| 管理员添加的中转渠道、Key、模型、参数转译与存储配置 | 数据库 `settings.private`，AES-GCM 加密 | 更新不得用镜像默认值覆盖已保存配置，保留渠道 ID |
| 数据加密密钥、会话签名密钥、站点与容器运行配置 | 运维保管的环境配置，运行时注入 | 不放镜像或 Git；独立备份并在换容器时沿用 |

新增第三方 API 若兼容已有协议，管理员或用户保存配置即可，不必重建镜像。若需新的请求/轮询/结果解析适配，更新镜像中的适配代码，已有地址、Key 和模型等配置仍保存在数据库；任何配置结构变更也属于迁移范围。此说明不代表已加入某个未指定的供应商。

`DATA_ENCRYPTION_KEY` 丢失或被重新生成后，即使数据库还在，既有 API Key 也无法正常解密。密钥轮换必须作为单独迁移处理。`JWT_SECRET` 的变更可能使原登录会话失效，不应随日常更新随机重置。

现状边界：repository/db.go 仍在启动时调用 GORM AutoMigrate，**尚未实现完整版本化迁移及跨版本回滚保护**。已完成的 CI 包括同版本重启恢复；尚未完成销毁旧容器后用另一应用版本恢复全部业务数据，以及数据库结构升级的端到端演练。不能把目录持久化等同于跨版本迁移已验收。

发布前需对数据副本执行目标版本迁移，核对账号、个人配置及管理员渠道解密、画布/任务归属、媒体数量/哈希和在途任务。SQLite 使用 WAL，不能在持续写入时只复制主 .db 文件作为完整备份；使用 SQLite Backup API，或停写/停应用后做一致性备份。媒体也须保持对应快照。备份放在应用不可写位置，恢复演练必须验证数据与独立保存的密钥可配套使用。

正式更新时沿用同一数据目录及密钥，一次只运行一个负责迁移/写入该 SQLite 库的应用实例。若数据库结构已经改变，不能仅把镜像切回旧版；回滚需使用已验证的兼容方案或对应数据备份，并处理切换后的新写入。容器持久化不代替备份。

## 风险计数口径

按用户要求，`GO-2026-5932` 对本次 Image ID 的有效风险计数为 0：编译依赖不包含受影响的 openpgp 子包，govulncheck 只有模块匹配，Node/Go 二进制与已核验版本一致。`evidence/runtime-candidate-ci.json` 分别记录 `scanMatches`（原始记录）与 `effectiveScanMatches`（复核后计数）；原始 JSON/SBOM 不过滤。排除只对已核验构建有效，相关依赖/代码变化后重新检查。原 Debian 的 25 个待跨发行版核对的 CVE 不因这个排除规则自动清除，也不能将本次有效扫描计数解释为全项目所有风险为零。

## 隔离测试实例的操作顺序

只有 deployment.md 对应阶段的前置条件满足后才执行。下面不自动连接或修改生产服务器，不安装运行时，不生成防火墙规则。

1. 在隔离构建机从已审查提交构建镜像，执行仓库 CI 的测试、最终镜像扫描和 SBOM 盘点。记录提交、Image ID、平台、基础摘要、扫描日期。依赖更新需要重新生成这些证据。
2. 通过可信通道将该镜像交给部署机；若以 archive 传输，校验 archive SHA256 后 `docker image load --input <已验证归档>`。若使用私有 registry，先取得发布授权，校验签名和完整 digest 后显式 pull。运行 Compose 不自行发布或下载镜像。
3. 使用独立测试数据目录和未占用端口。复制 .env.example 到运维专用文件，权限 0600，填写随机密钥和已确认限额；配置与数据分别备份。数据目录权限应限制到应用身份。Rootless 的 UID 是映射后的宿主机 UID，不能照抄宿主机 chown 10001；先验证映射和可写性。不要对现有数据目录递归改属主。
4. 根据实际路径运行配置检查与 create，先不启动应用。`config --quiet` 不打印含密钥的完整配置。示例中的运维变量必须先按真实验收记录设置，不能从待检查容器自身推导出“期望镜像”。

```sh
docker compose -p ican-staging --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml config --quiet
docker compose -p ican-staging --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml create --no-build app
ICAN_CONTAINER_ID=$(docker compose -p ican-staging --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml ps -aq app)
python3 scripts/check-container.py "$ICAN_CONTAINER_ID" \
  --image-id "$ICAN_REVIEWED_IMAGE_ID" \
  --data-dir "$ICAN_REVIEWED_DATA_DIR" \
  --host-port "$ICAN_REVIEWED_HOST_PORT"
```

5. 检查成功且主机网络规则已通过验收后，才执行同组 Compose 参数的 `start app`。使用本地 health 接口和测试账号验收，再核对实际进程 Seccomp=2、NoNewPrivs=1、CapEff=0，以及重启后账号/素材恢复。
6. checker 只做只读配置核对，不打印 Env/密钥，也不会修改 Docker 或防火墙。通过结果仍明确 `productionApproved: false`，不证明有效网络隔离、内核补丁、浏览器功能或迁移完成。

## 服务器验收与回滚

先盘点现有服务、管理通道、内核和 Docker/containerd/runc 版本、Rootless/cgroup/seccomp 支持与防火墙后端。宿主机补丁与容器依赖补丁分别处理，不因为容器扫描无高危就跳过宿主机。

出站规则必须覆盖宿主机地址、其他容器/局域网、云元数据及 IPv6，同时保留必要 DNS、用户明确需要的公共模型 API 和经授权的存储目的地。Docker iptables 的转发规则与宿主机 INPUT 都需检查；Rootless/nftables 路径不同，不能直接复制 DOCKER-USER 规则。禁止清空防火墙、停掉已有 Docker 服务或切断唯一管理入口。

Nginx/TLS、旧公开素材路由、日志脱敏、账号隔离、外部 API 参数/历史任务、原数据迁移与恢复演练全部验收后，才适用用户的条件部署授权。API Key 在运行时仍对应用可用；容器隔离不能保护已被攻破进程自己持有的秘密。

更新时构建和验证新镜像，保留旧镜像摘要、对应配置及一致性数据备份，再替换容器。不得在运行容器内 apt upgrade、下载补丁脚本或原地改二进制。容器删除不应删除数据目录；不要运行清理业务数据的 prune/down -v 指令。数据库结构有变化时，不能只换回旧镜像，必须按 deployment.md 的数据回滚流程处理切换后的新写入。

参考：[Docker 安全](https://docs.docker.com/engine/security/)、[Rootless](https://docs.docker.com/engine/security/rootless/)、[Compose 服务配置](https://docs.docker.com/reference/compose-file/services/)、[镜像构建与更新](https://docs.docker.com/build/building/best-practices/)、[glibc-dynamic](https://images.chainguard.dev/directory/image/glibc-dynamic/overview)。
