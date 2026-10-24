# 运行组件与镜像审核

## 审核方法与范围

对实际镜像执行 Trivy 0.76.1 扫描，保留所有严重度和安装包清单；不使用 ignore、忽略未修复项或依赖分类调整。扫描程序从官方发布下载并核验 SHA256。扫描报告是公告匹配，不是漏洞利用证明，也不覆盖未知零日或所有静态链接的原生库。

业务运行版本为 Node 24.22.1、Go 1.27.3、Next 16.4.3、Sharp 0.35.5；版本与摘要在 evidence 中。完整源码/锁文件 OSV 查询与 govulncheck 结果见 hardening-review.md。

## 实际减少运行组件

最初采用的 Node Bookworm slim 最终应用镜像在 CI 完成构建、UID 10001、只读根目录、断网运行、私有媒体及重启恢复检查，但漏洞步骤阻断：56 HIGH、1 CRITICAL 公告匹配，均为 Debian 组件；npm 包管理器已不在运行层，应用 Node 包没有匹配。报告及失败记录保留，不将失败改写为通过。

复核以下官方公告发现，多条记录对应相同源码包，且触发前提不同：

| 公告 | 触发条件与本次处置 |
|---|---|
| [CVE-2023-45853](https://security-tracker.debian.org/tracker/CVE-2023-45853) | MiniZip 的 ZIP 写入；Debian 明确说明 Bookworm zlib 二进制未构建对应 contrib 代码。不能把 zlib1g 匹配直接称为本应用可利用漏洞。 |
| [CVE-2026-16742](https://security-tracker.debian.org/tracker/CVE-2026-16742) | systemd-homed 本地用户提权；应用不启动 homed，后续运行层完全不携带 systemd 组件。 |
| [CVE-2025-69720](https://security-tracker.debian.org/tracker/CVE-2025-69720)、[CVE-2026-9538](https://security-tracker.debian.org/tracker/CVE-2026-9538) | infocmp 命令及 Perl Archive::Tar 处理恶意输入；业务无调用，后续运行层移除这些工具。 |
| [CVE-2026-41992](https://security-tracker.debian.org/tracker/CVE-2026-41992)、[CVE-2026-54369](https://security-tracker.debian.org/tracker/CVE-2026-54369) | gzip LZH 解压与特权 libacl 路径操作；业务无此功能，后续不携带这些工具/库。 |
| [CVE-2026-53613](https://security-tracker.debian.org/tracker/CVE-2026-53613)、[53615](https://security-tracker.debian.org/tracker/CVE-2026-53615)、[76642](https://security-tracker.debian.org/tracker/CVE-2026-76642)、[78408](https://security-tracker.debian.org/tracker/CVE-2026-78408)、[78409](https://security-tracker.debian.org/tracker/CVE-2026-78409)、[78410](https://security-tracker.debian.org/tracker/CVE-2026-78410) | util-linux 的挂载、分区或命名空间操作；应用不需要这些能力，后续运行层移除对应工具/库，同时保留非 root、无 capabilities、no-new-privileges。 |

单纯改用 Node Trixie slim 仍有 43 个系统包 HIGH 匹配，因此未采用。最终改为官方 `gcr.io/distroless/cc-debian13:nonroot`，固定摘要，只复制已核验的 Node 可执行文件、Go 程序和 Next standalone 产物。应用仍是原来的 Go + Next 两个进程；使用 Node 内置进程 API 转发终止信号、收割子进程并在任一子进程退出时停止容器，没有新增服务。运行镜像没有 shell、apt、npm、Yarn、Perl、mount 或 nsenter。

Distroless 摘要通过 Cosign 3.1.4 验证：OIDC issuer 为 `https://accounts.google.com`，证书身份为 `keyless@distroless.iam.gserviceaccount.com`，检查证书、签名及透明日志。证据在 `evidence/distroless-signature.json`。

## 最终应用镜像验证

提交 `a98014b6e1add88b35ce2587bac4cd8f09a81b4c` 的 [CI 40081455482](https://github.com/ruoyeyu/ICAN/actions/runs/40081455482) 已通过：Go 测试/构建、18 项前端测试/生产构建、UID 10001、只读根目录、有效 capabilities 为零、NoNewPrivs、断网、私有媒体/授权撤销、登录退出、重启恢复及 Node/Sharp 加载。实际扫描范围为 14 个 Debian 包、19 个 Node 包和 Go 二进制中的 49 个模块。最终扫描为 HIGH/CRITICAL 0、MEDIUM 23、LOW 8、UNKNOWN 1；未知严重度是 x/crypto 的 openpgp 公告，本程序不导入该包，源码可达性证据已保留。

完整报告位于 `evidence/final-image-vulnerabilities.json.gz`，由同一报告离线转换的 CycloneDX SBOM 为 `evidence/final-image-sbom.cdx.json.gz`。CI 高危门槛通过不等于所有中低危问题、原生库或生产环境已获准。

## 保留的风险与证据

Distroless 基础层有 14 个包，扫描仍记录 23 MEDIUM、8 LOW 匹配。包括 glibc、libstdc++、zlib 的特定输入/函数问题。它们没有被隐藏；不能仅凭严重度较低就认定不影响应用。后续需结合最终包清单和调用路径持续复核，尤其是解析器、DNS、字符串转换及静态链接组件。主 Go 程序 CGO_ENABLED=0；Node 使用的内置库和 Sharp 自带原生库不能仅按 Debian 包版本判断。

Sharp 及其原生库已实际加载，Next 图片优化端点已关闭并验证拒绝访问。扫描工具对 npm 包的识别不能代替对 Sharp 静态捆绑组件的逐项审核。没有在生产进行畸形媒体或资源耗尽测试。

完整原始扫描以 `.json.gz` 保存，可用 `gzip -dc` 解压；没有从报告中删掉告警。最终镜像运行与扫描记录见 `evidence/image-ci.json`。镜像未推送至注册表，因此记录的是 CI 构建 Image ID，不是一个已发布镜像的 RepoDigest。

即使 CI 的高危门槛通过，也不代表生产部署门槛全部通过。上传/容量/并发边界、宿主机隔离、真实画境合同、旧数据迁移及备份回滚仍按 deployment.md 单独阻断上线。
