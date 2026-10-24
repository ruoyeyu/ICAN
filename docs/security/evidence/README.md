# 验证证据

基线 e13006fbaeabe14e4a6d27cba667ff1473009ebc。

- go-tests.jsonl：177 个测试及子测试通过（含原有测试）；不是 177 个漏洞。
- web-tests.log：17 个前端合同测试通过。
- production-build.log：Next 生产编译与 TypeScript 检查通过。
- runtime-smoke.json：17 项生产 Go + Next 进程联通检查。
- resolved-dependencies.json、osv-results.json、osv-details.json：569 项完整依赖图查询和原始公告。
- govulncheck.json：Go 源码可达性分析；模块级匹配不等于漏洞函数可达。
- go-runtime-packages.txt、go-module-paths.txt：主程序编译依赖与不相关模块的路径证据。
- native-runtime.json：实际加载的 Node/Sharp/原生库版本，不等于所有库完成 CVE 审核。
- base-images.json、toolchain-verification.json：上游镜像摘要及工具链校验来源。

所有调用均为隔离 fixture、本地进程或公开依赖查询，没有使用真实供应商 Key 或执行付费生成。最终容器和生产服务器不在本地联通测试范围。
