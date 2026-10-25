# 服务器只拉镜像运行

ICAN 的 Go 后端、Web 前端和导演台全部由 GitHub Actions 编译。服务器不需要 Go、Node、Bun，也不执行 `docker build`。当前成品平台为 **Linux / amd64**。

## 本次已发布成品

[流水线 40179792350](https://github.com/ruoyeyu/ICAN/actions/runs/40179792350) 的构建、检查及发布全部通过。源码 commit：`e03fc098cca2efdbac74a9e85f61b81692f9d4f9`。

```sh
docker pull ghcr.io/ruoyeyu/ican@sha256:7c67aebfd53e60b2bbcdf5eed80b68742a756c9e5ca22bd5b3f6a601305d5580
```

[镜像及检查记录](evidence/ghcr-v0.1.3-publication.json) · [完整原始安全证据](evidence/ghcr-40179792350-evidence.zip)。

上述镜像 manifest 已验证可匿名获取且 digest 一致，无需登录 GitHub。

## 取得经过检查的成品

打开成功的 Actions 运行记录，查看 Summary 或下载 `published-image` 中的 `publication.json`，取得完整的：

- 源码 commit。
- `ghcr.io/ruoyeyu/ican@sha256:…` 镜像地址。
- 镜像 ID、运行平台和检查结果链接。

`image-security-evidence` 提供原始扫描、SBOM、原生组件清单和 Compose 检查记录。只有所有前置检查成功才允许发布；发布加载同一份已检查镜像，不重新编译。PR 不发布，只有本仓库 main 的受支持运行才发布。镜像标签包含 commit 和运行编号，但部署仍必须固定 digest，不能使用浮动标签。

GHCR 首次创建的包可能是私有的。私有包需要服务器使用只读包凭据登录；公开包可匿名拉取。凭据只由运维保管，不放入项目、镜像或应用环境。Actions 自身使用临时 `GITHUB_TOKEN`，不需要提供服务器密码、SSH Key 或个人 API Key。

## 准备运行配置

从发布记录对应的 commit 取得 `docker-compose.yml`、`.env.example` 和 `scripts/check-container.py`。保管好原有配置；首次测试才新建独立数据目录和初始密钥。

Compose 固定 **CPU 1.6 核、内存 1536 MiB（1.5 GiB），不额外使用 swap**。这些是运行限制，不限制 GitHub 编译。其余 PID、临时目录大小、登录速率和供应商链接有效期仍须由运维明确填写。内存超限可能使容器被终止，容器限额不能代替尚待完成的应用资源边界。

将配置保存在仓库外的受保护环境文件（权限 0600），填写镜像 digest、站点来源、回环端口、已有数据绝对路径和应用配置。不要直接打印含密钥的完整 Compose 配置或 Docker inspect 输出。数据目录必须提前存在，权限应与容器身份匹配；Rootless Docker 还需核对 UID 映射。

以下操作由服务器运维执行；本任务不会自动执行它们。路径和端口是示例，须替换为已核对的实际值：

```sh
ICAN_OPERATOR_ENV=/secure/operator/ican.env
ICAN_VERIFIED_IMAGE='ghcr.io/ruoyeyu/ican@sha256:<发布记录中的完整digest>'
ICAN_REVIEWED_DATA_DIR=/srv/ican/data
ICAN_REVIEWED_HOST_PORT=3001

docker pull "$ICAN_VERIFIED_IMAGE"
ICAN_REVIEWED_IMAGE_ID=$(docker image inspect "$ICAN_VERIFIED_IMAGE" --format '{{.Id}}')
# 与 publication.json 核对镜像 ID 和源码 commit。
docker image inspect "$ICAN_VERIFIED_IMAGE" --format '{{.Id}} {{index .Config.Labels "org.opencontainers.image.revision"}}'

docker compose -p ican --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml config --quiet
docker compose -p ican --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml create --no-build app
ICAN_CONTAINER_ID=$(docker compose -p ican --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml ps -aq app)
python3 scripts/check-container.py "$ICAN_CONTAINER_ID" \
  --image-id "$ICAN_REVIEWED_IMAGE_ID" \
  --data-dir "$ICAN_REVIEWED_DATA_DIR" --host-port "$ICAN_REVIEWED_HOST_PORT"
# 检查成功、宿主机隔离条件满足后启动。
docker compose -p ican --env-file "$ICAN_OPERATOR_ENV" -f docker-compose.yml start app
curl --fail "http://127.0.0.1:$ICAN_REVIEWED_HOST_PORT/api/health"
```

环境文件中的 `ICAN_IMAGE` 必须与核对过的 digest 一致。Compose 禁止隐式拉取；先明确 `docker pull`，再创建容器。端口仅绑定 127.0.0.1，对外代理、TLS 和防火墙仍由服务器运维单独验收。

## 更新和回滚

v0.1.3 不增加数据库列；旧账号、渠道 ID、配置和素材沿用。新表单规则含旧版不支持的原始参数变量，回退 v0.1.2 必须同时恢复更新前的配置或匹配备份，不能只换旧镜像。旧版程序创建测试数据后，备份副本升级、重启和恢复旧版已验证，见 [本地证据](evidence/api-config-ui-local.json)。生产数据和期间新增内容仍需单独验收与保留。

1. 先拉取新 digest，保留旧 digest 及对应 Compose、配置和密钥。不要删除正在使用或回滚需要的 GHCR 镜像版本。
2. 先在数据副本验收新版本。维护窗口停止应用写入后，备份整个数据目录（SQLite 主库、WAL/SHM、媒体等），另外安全备份环境配置和密钥。备份放在应用不可写的位置，验证能够恢复和解密；不能只复制运行中的 `.db`。
3. 更新只修改环境文件的 `ICAN_IMAGE`。保持 `ICAN_DATA_DIR`、`DATA_ENCRYPTION_KEY`、`JWT_SECRET` 不变，再按上面流程重建、检查和启动。账号、用户配置、API/渠道配置和素材索引保存在原数据库，媒体文件仍在原目录。
4. 若失败，先停止新版本，另存失败后的数据。旧版确认兼容现有结构时，才可直接换回旧 digest；兼容性不明时，将升级前的完整备份恢复到另一个隔离目录，使用匹配的旧镜像、配置和原密钥运行。保留失败目录，不覆盖；备份之后的新写入需要另行处理。

不要重新生成密钥、清库、用空目录替换原数据，也不要让新旧实例同时写同一 SQLite 库。未同步的浏览器本地数据不包含在服务器备份中。

冷备份命令及详细发布策略见 [完整说明](ghcr-deployment.md)。测试副本的本次升级/回滚已验证；生产数据恢复及后续结构迁移仍需验收；“发布成功”表示成品通过本次自动化检查，**不表示资源边界、会话凭据及生产部署验收全部完成**。参见 [上线阻断项](../progress/todo.md)。
