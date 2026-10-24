# 中低危、原生二进制与外部引用复核

本轮针对上一版最终镜像的 23 MEDIUM、8 LOW 记录复核，同时检查固定第三方地址、可变远程代码、预编译组件与最终打包内容。采用的上游仍为 `e13006fbaeabe14e4a6d27cba667ff1473009ebc`。原始扫描不删记录、不改严重度、不用 ignore 制造通过。

## 结论边界

31 条系统包匹配去重后为 25 个 CVE；两个 GCC 公告分别匹配四个同源码包的二进制包。它们不是 31 个已证实可从网站利用的漏洞，也不能因为没有复现就称全部无影响。稳定 Debian Trixie 的 glibc/GCC 当前仍列出这些问题，多项标记 no-dsa；测试/不稳定发行版出现修复不等于可直接混装到当前镜像。保留稳定发行版和安装元数据，继续追踪官方补丁；没有私自替换系统 libc 或仅删除包数据库。

本轮已经移除未使用的 Sharp/libvips 运行产物。Next 图片优化原先已禁用；构建仍保留锁定且审查过的依赖，不通过更改依赖分类规避扫描。生产 standalone 验证不再携带 `sharp`、`@img` 或其 `.node`/libvips 文件。移除的是服务器可执行解析器，不影响浏览器显示用户原始图片；图片/视频实际浏览器交互仍需产品验收。

## 逐项公告与触发条件

完整原始公告表格与备注保存在 `evidence/second-pass-advisories.json`，下表各链接为 Debian 官方追踪器，内含对应上游修复/讨论。已安装版本：libc6 `2.41-12+deb13u4`；GCC 系包 `14.2.0-19`；zlib1g `1:1.3.dfsg+really1.3.1-1+b1`。以下“未见调用”只表示源码及动态符号检查结果，不能证明所有静态/间接路径均不可达。

| CVE | 触发前提及当前处置 |
|---|---|
| [2026-102010](https://security-tracker.debian.org/tracker/CVE-2026-102010) | libstdc++ PBDS 二叉堆 `erase_if` 悬空指针；没有发现应用调用，模板实例化不能只靠共享库版本判断。保留匹配，等待稳定版补丁。 |
| [2026-95619](https://security-tracker.debian.org/tracker/CVE-2026-95619) | C++ aligned new 超大分配溢出；原 libvips 引入 aligned array-new，现移除；Node 动态导入未见该入口。不是对所有 C++ 静态路径的完整证明。 |
| [2026-18374](https://security-tracker.debian.org/tracker/CVE-2026-18374) | 攻击者控制 `fopen` mode 中空 `,ccs=`；应用未提供该 mode 参数入口，保留底层匹配。 |
| [2026-19499](https://security-tracker.debian.org/tracker/CVE-2026-19499) | `strfmon[_l]` 特定宽度补齐及过小目标缓冲；未见 Node/应用导入或调用。 |
| [2026-19542](https://security-tracker.debian.org/tracker/CVE-2026-19542) | `tdelete` 特定深度的大树；未见应用调用，未运行百万节点资源耗尽复现。 |
| [2026-5435](https://security-tracker.debian.org/tracker/CVE-2026-5435) | 已弃用 DNS 打印调试函数处理 TSIG；不是普通 DNS 查询路径，未见 Node 导入。 |
| [2026-6238](https://security-tracker.debian.org/tracker/CVE-2026-6238) | 同组 DNS 调试打印函数解析 RDATA；未见调用，不能等同所有 DNS 均存在该问题。 |
| [2026-6368](https://security-tracker.debian.org/tracker/CVE-2026-6368) | `wordexp` 的 WRDE_APPEND；未见应用导入，运行镜像不带 shell。 |
| [2026-6791](https://security-tracker.debian.org/tracker/CVE-2026-6791) | `wordexp` 展开超长 `~用户名`；未见应用路径。 |
| [2026-77117](https://security-tracker.debian.org/tracker/CVE-2026-77117) | SHIFT_JISX0213 iconv 小输出缓冲循环；移除会导入 iconv 的 libvips；Node 的文字处理不能仅因同名功能就认定调用 glibc iconv。 |
| [2026-80489](https://security-tracker.debian.org/tracker/CVE-2026-80489) | EUC_JISX0213 对应转换循环；处置同上。 |
| [2026-8674](https://security-tracker.debian.org/tracker/CVE-2026-8674) | 约 200 字符的 DNS search 域，来自 resolv.conf/LOCALDOMAIN，可导致解析器退出。**仍需宿主机 DNS 配置验收**；只读应用根目录不能代替对 Docker 注入 DNS 配置的检查。 |
| [2026-86805](https://security-tracker.debian.org/tracker/CVE-2026-86805) | setuid/setgid AT_SECURE、特定 `$ORIGIN` 路径及硬链接竞争；镜像检查 set-ID 文件、非 root、cap-drop 和 no-new-privileges，排除当前镜像内提权前提。宿主机另外审计。 |
| [2026-95818](https://security-tracker.debian.org/tracker/CVE-2026-95818) | 同类 AT_SECURE 动态装载器路径缓冲溢出；当前容器不需要特权 ELF，检查同上。 |
| [2026-89092](https://security-tracker.debian.org/tracker/CVE-2026-89092) | 必须运行 nscd 守护进程并处理恶意 DNS 大响应；当前应用只启动 Node 和 Go，不运行 nscd，组件清单不含 nscd 包。 |
| [2026-97399](https://security-tracker.debian.org/tracker/CVE-2026-97399) | Power8 专用 strncasecmp；本轮构建/验收为 linux/amd64，不适用于该架构。其他架构需重审。 |
| [2010-4756](https://security-tracker.debian.org/tracker/CVE-2010-4756) | libc glob 资源消耗；Debian 注明 POSIX 行为需调用方限制，未见应用使用该 libc 接口。整体容量边界仍独立阻断上线。 |
| [2018-20796](https://security-tracker.debian.org/tracker/CVE-2018-20796) | libc POSIX regex 深递归；应用的 JS/Go regex 不等同该实现，未见 Node 动态导入；上游不作为安全漏洞处理。 |
| [2019-9192](https://security-tracker.debian.org/tracker/CVE-2019-9192) | 同类特制 POSIX regex；上游有争议，保留记录和适用性说明。 |
| [2019-1010022](https://security-tracker.debian.org/tracker/CVE-2019-1010022) | 先有栈溢出再绕过栈保护；上游不作为独立安全问题，不能据此保证不存在其他内存问题。 |
| [2019-1010023](https://security-tracker.debian.org/tracker/CVE-2019-1010023) | 对恶意 ELF 执行 ldd；应用不执行上传 ELF，也不提供 ldd。二进制盘点使用 readelf，只解析不执行。 |
| [2019-1010024](https://security-tracker.debian.org/tracker/CVE-2019-1010024) | 线程栈/堆缓存帮助绕过 ASLR；上游不作为独立安全问题，保留原记录。 |
| [2019-1010025](https://security-tracker.debian.org/tracker/CVE-2019-1010025) | 线程堆地址可预测帮助绕过 ASLR；同上。 |
| [2026-27171](https://security-tracker.debian.org/tracker/CVE-2026-27171) | zlib `crc32_combine64`/`crc32_combine_gen64` 循环；系统库版本仍匹配，Node 并非动态链接此系统 zlib。Node 内置版本单列盘点，不能借系统包扫描覆盖它。 |
| [2026-85091](https://security-tracker.debian.org/tracker/CVE-2026-85091) | 非阻塞 gzwrite 停滞后 gzprintf/gzvprintf 缓冲溢出。公告描述从 1.3.1.2 引入，但 Debian 仍标记 1.3.1 受影响；**范围存在矛盾，尚未完成 Debian 源包回移补丁核对，不自行改成误报**。未见 Node 动态链接系统 zlib 或业务调用 gz*。 |

另有 UNKNOWN `GO-2026-5932`：模块为 x/crypto，涉及未导入的 openpgp；保留源码 govulncheck 与运行包清单证据，不把模块匹配当成 bcrypt 可利用问题。

## 二进制来源

- Git 跟踪源码未发现 ELF、PE、Mach-O、WASM 魔数的可执行 blob。检查不等于证明没有混淆/编码载荷。
- 最终 Go 服务由仓库源码、go.sum 固定依赖、CGO_ENABLED=0 构建；SQLite 为纯 Go 路径，不下载闭源驱动。
- Node 为官方镜像的开源预编译二进制；最终 Debian glibc/libstdc++ 等来自签名验证且摘要固定的 Distroless。保留 Node license 和镜像原有版权文件。
- 构建期 Bun、Next SWC/Tailwind 原生包、原 Sharp/libvips 都有公开源码和开源许可证；“预编译”不等于“闭源”。未对这些上游发布品进行完整可复现构建比对，因此不宣称可排除上游编译链投毒。
- Sharp/libvips 及其捆绑字体、XML、各类图片编解码库不再进入最终运行层。浏览器使用的系统/WebCodecs 编解码器是客户端平台组件，不在本服务器镜像审核范围。
- `scripts/inventory-image.py` 从最终镜像 tar 流读取文件魔数、SHA256、动态依赖/未定义符号、set-ID 位和许可证路径；不执行被检查的 ELF。清单与 SBOM 一并作为 CI artifact 保存。静态内置 Node 库另用 `process.versions` 记录，SBOM 不能覆盖全部静态组件的函数可达性。

## 外部引用发现与整改

| 发现 | 触发条件 | 修改 |
|---|---|---|
| `kieai.redpandaai.co/api/file-stream-upload` | 旧 KIE 适配器处理本地参考素材时，向固定第三方上传，并携带渠道 Key；与用户自设 BaseURL 不一致 | 删除服务端发送逻辑及前端上传指令生成，明确报错；本地 fixture 断言零网络调用。用户自设 API/服务器素材授权链接保留。未据此认定上游恶意。 |
| TokenDance 全局模型目录 | 旧协议自动获取第三方模型协议 | 移除目录网络实现，后台保存/模型测试/参数转译入口统一拒绝已关闭协议。 |
| 上游 `main` 的 VERSION/CHANGELOG 检查 | 已未挂载的旧 hook 仍含可变源 | 删除 hook；无自动更新执行器。 |
| GitHub `main` 远程提示词仓库 | 原提示词订阅服务 | 删除下载器及调度实现，旧同步入口明确不可用；用户自己的提示词数据保留。 |
| AutoDL/RunningHub 默认地址及本地 ComfyUI 固定上传端口 | 已关闭模块中的默认网络目标 | 清除默认地址、上传请求和上游插件安装命令；后端关闭状态保留。 |
| 原仓库附带 Agent marketplace/skills/source 锁定不足 | 开发辅助工具可能拉取另一个分支的指令/插件 | 删除无关分发内容及 skills-lock；应用无需这些内容运行。 |

自有前端没有远程 CDN JS/CSS/字体导入；新增 CSP 将脚本、样式和字体来源限定到本站（内联样式/脚本及字体 data 例外）。由于现有参数转译 worker 使用动态函数，仍保留 unsafe-eval/unsafe-inline，不把 CSP 宣称为完整 XSS 防线。外部图片/媒体以及用户自定义 API 的 connect 仍允许，这是产品需要；供应商结果内容可能随远端变化，服务端仍执行出站地址与媒体内容验证。

保留的 Linux.do/本地 Agent/Bridge 源码只用于未启用模块，路由测试确认拒绝访问；Linux.do 默认 URL 不是此次运行中的后台订阅。API 代理中的 127.0.0.1 是容器内部 Next→Go 通信地址，不是给普通用户的任意内网访问例外。

## 哪些内容仍可更新

- package.json 直接依赖改为当前锁文件的精确版本；本轮锁文件解析后的整个 packages 图保持不变。Bun `--frozen-lockfile --ignore-scripts`、go.sum 与 `go mod verify`、`GOTOOLCHAIN=local` 阻止无意漂移或自动换工具链。
- Docker 构建和运行镜像都固定 digest；GitHub Actions 固定完整 commit SHA；扫描器固定版本与下载 SHA256。构建不运行上游一键脚本，不发布镜像或部署站点。
- CI 改为 ubuntu-24.04 标签；GitHub 托管 runner 的补丁仍会更新，标签不是不可变宿主机。Trivy/OSV 漏洞数据库也会更新，它们是审计数据，非应用远程代码。结果必须绑定扫描时间和镜像摘要重审。
- 用户配置的 API/媒体/存储服务、网络 DNS/TLS/软件仓库基础设施仍是外部依赖；固定源码不意味着外部服务可信。依赖更新要新提交、重跑测试/镜像盘点与审查，不能让运行容器自更新。

## 验证、限制与回滚

本地 179 个 Go 测试及子测试通过，新增固定上传阻断、关闭协议和关闭自定义渠道回归；18 个前端测试及生产构建通过，19 项 Go/Next 实际进程联通验证通过，standalone 无 Sharp/@img。最终镜像 CI 结果随后写入 `evidence/second-pass-ci.json`。生产未部署，未付费调用。真实浏览器 CSP/媒体体验、DNS 配置、容量并发边界、画境合同、旧数据和回滚演练仍是上线门槛。

本轮无数据结构修改。可在独立测试实例回退到上一候选提交 `9f1a91e53c6ccc11c5918ecb4241e65c07b6cf3f` 复现对比，但会恢复此次移除的第三方上传和 native 组件，不建议作为生产回退目标；线上原站保持不变。数据库/密钥/用户媒体的生产备份回滚仍按 deployment.md 执行。
