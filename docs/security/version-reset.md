# Complete the v0.1.0 label reset / 完成版本标签重置

The application image is already built, tested and published. [Checks and digest](evidence/ghcr-v0.1.0-restart-publication.json). The Release step stopped because GitHub refused the Actions token's tag update across workflow changes; the atomic update changed neither tag. [Failure record](evidence/version-reset-blocked.json).

源码、镜像和检查已经完成。当前只阻断版本标签：Actions 令牌缺少本次操作要求的 workflows 权限，现有连接没有标签管理接口。没有修改服务器，也没有绕过检查。

A repository owner or an authorized local agent can run the following from an existing Git checkout using their normal GitHub SSH/OAuth credentials with the necessary repository/workflow permissions. Do not paste tokens into these commands, the repository or application. This updates only the two explicitly approved version refs; old commits and GHCR images are retained.

仓库所有者或已有相应权限的本地 Agent，可在 Git 工作目录中使用自己的正常 GitHub 凭据执行以下命令，不需要服务器编译，也不要将 Token 写入命令、仓库或应用。

```sh
git fetch https://github.com/ruoyeyu/ICAN.git 53f8aa31d8fd803668cc138b3d91262ea9903c89
git push --atomic \
  --force-with-lease=refs/tags/v0.1.0:4702d09c590769f5ec73ef3de6b312702bcea7e3 \
  --force-with-lease=refs/tags/v0.1.3:4cef8067138572047233b72e0fe67373bc1d3242 \
  https://github.com/ruoyeyu/ICAN.git \
  53f8aa31d8fd803668cc138b3d91262ea9903c89:refs/tags/v0.1.0 \
  :refs/tags/v0.1.3
```

The leases require the exact observed old tag values. If either changed, stop and inspect; do not replace this with an unrestricted force push. This moves v0.1.0 to the checked source and removes the discontinuous v0.1.3 label in one transaction. No branch is rewritten.

旧值校验不匹配时应停止核对，不改成无条件强推。命令一次性将 v0.1.0 指向已验收源码，并删除不连续的 v0.1.3 标签，不改写主线或删除历史提交。

Then run **Actions → Publish checked pre-release → Run workflow → main**. The regular workflow repeats the source-CI, source-version, exact tag, image digest/labels and evidence checks before publishing all six checksum-verified assets. It does not rebuild or deploy a server. The GitHub image pipeline may run independently for later repository maintenance commits; the release remains bound to the explicit checked source above.

完成后运行 **Actions → Publish checked pre-release → Run workflow → main**。正常发布流程会再次核对成功 CI、源码版本、标签、镜像 digest/标签和证据，全部通过才发布六个校验附件。不会重编译或部署服务器。

Until then, use the existing immutable image through the [deployment guide](ghcr-deployment.md) / [中文部署说明](ghcr-deployment.zh-CN.md); do not use the old v0.1.0 tag as the new source.

## Merged historical branches / 已合并的历史分支

A final remote check found two remaining historical branches. Both tips are ancestors of current `main`; neither was modified by the failed reset. To finish the previously requested main-only cleanup, the same authorized owner/local agent can delete only these exact tips:

最终远端核对仍有两条历史分支，已验证其提交都包含在当前主线中。完成此前要求的“仅保留主线”清理，可由同一所有者或本地 Agent 删除以下精确分支，不影响主线代码：

```sh
git push --atomic \
  --force-with-lease=refs/heads/security/ican-hardening:bd112957b2121fdbc80a4eaffab50e61bcde92af \
  --force-with-lease=refs/heads/security/runtime-isolation-candidate:bad3a1b93df6c671d985f76ccf88498171ed7dbc \
  https://github.com/ruoyeyu/ICAN.git \
  :refs/heads/security/ican-hardening \
  :refs/heads/security/runtime-isolation-candidate
```

If a tip changed, stop and recheck ancestry; do not remove the lease protection.
