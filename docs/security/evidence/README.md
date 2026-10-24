# 验证证据

基线 e13006fbaeabe14e4a6d27cba667ff1473009ebc。

- go-tests.jsonl：177 个测试及子测试通过（含原有测试）；不是 177 个漏洞。
- web-tests.log：18 个前端合同测试通过。
- production-build.log：Next 生产编译与 TypeScript 检查通过。
- runtime-smoke.json：17 项生产 Go + Next 进程联通检查。
- resolved-dependencies.json、osv-results.json、osv-details.json：569 项完整依赖图查询和原始公告。
- govulncheck.json：Go 源码可达性分析；模块级匹配不等于漏洞函数可达。
- go-runtime-packages.txt、go-module-paths.txt：主程序编译依赖与不相关模块的路径证据。
- native-runtime.json：实际加载的 Node/Sharp/原生库版本，不等于所有库完成 CVE 审核。
- base-images.json、toolchain-verification.json：上游镜像摘要及工具链校验来源。
- distroless-signature.json：固定运行基础镜像摘要的 Cosign 签名、身份与透明日志核验结果。
- *vulnerabilities.json.gz：基础镜像比较和最终应用镜像的完整 Trivy 报告，包含安装包与全部严重度，未删减告警。
- final-image-sbom.cdx.json.gz：从最终扫描报告转换的 CycloneDX 组件清单。
- image-ci.json：镜像构建、运行隔离、重启恢复与扫描的远端 CI 证据索引；详见 ../runtime-review.md。

所有调用均为隔离 fixture、本地进程或公开依赖查询，没有使用真实供应商 Key 或执行付费生成。最终容器在 GitHub 独立 runner 验证，生产服务器不在本次测试范围。
