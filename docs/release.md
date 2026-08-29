# OScraper 发布指南

OScraper 仅向 GitHub Container Registry（GHCR）发布镜像。普通功能分支 push 不触发 CI；面向 `main` 的 Pull Request、`main` push 和手动 CI 才运行测试。只有受支持的版本 Tag 会构建并上传镜像。

## 1. 首次配置

在 GitHub 仓库的 **Settings → Environments** 创建：

- `ghcr-beta`：不配置审批，Beta 自动发布。
- `ghcr-production`：配置 Required reviewers，正式镜像上传前必须审批。

工作流使用仓库自带的 `GITHUB_TOKEN`，只给发布 Job `packages: write` 权限，不需要额外 Secret。首次发布后，可在 GitHub Packages 中按需要把 `oscraper` 包设为公开；私有包拉取时需要具有 `read:packages` 的凭据。

建议通过 Repository Ruleset 保护 `v*` Tag，禁止修改或删除已发布 Tag，并限制正式版本 Tag 的创建者。

## 2. 版本与镜像标签

| 渠道 | Git Tag | 发布的镜像标签 |
| --- | --- | --- |
| Beta | `v1.2.0-beta.1` | `1.2.0-beta.1`、`beta`、`sha-<commit>` |
| 正式版 | `v1.2.0` | `1.2.0`、`1.2`、`1`、`latest`、`sha-<commit>` |

Beta 不会更新 `latest`、主版本或次版本标签。精确版本镜像一旦存在，工作流会拒绝覆盖。

仅接受以下格式：

```text
^v[0-9]+\.[0-9]+\.[0-9]+-beta\.[0-9]+$
^v[0-9]+\.[0-9]+\.[0-9]+$
```

Tag 对应的提交必须包含在 `main` 历史中，否则发布立即失败。

## 3. 发布 Beta

先确保目标提交已经合入 `main`：

```bash
git switch main
git pull --ff-only
git tag v1.2.0-beta.1
git push origin v1.2.0-beta.1
```

版本校验后，代码测试与单架构容器健康烟测并行执行。两者通过后，AMD64 与 ARM64 镜像分别在 GitHub 托管的原生架构 Runner 上并行构建，最终合并并核验多架构 Manifest。平台构建使用轻量缓存；缓存导出失败不会阻断已经完成的镜像构建。

```bash
docker pull ghcr.io/<repository-owner>/oscraper:1.2.0-beta.1
docker pull ghcr.io/<repository-owner>/oscraper:beta
```

部署测试环境时优先使用精确版本，`beta` 适合始终跟随最新 Beta 的环境。

## 4. 发布正式版

在通过验收的 `main` 提交上创建正式 Tag：

```bash
git switch main
git pull --ff-only
git tag v1.2.0
git push origin v1.2.0
```

测试和烟测通过后，发布 Job 会进入 `ghcr-production` 环境等待审批。审批通过后上传双架构镜像并更新正式标签：

```bash
docker pull ghcr.io/<repository-owner>/oscraper:1.2.0
docker pull ghcr.io/<repository-owner>/oscraper:latest
```

生产部署应固定精确版本：

```yaml
image: ghcr.io/<repository-owner>/oscraper:1.2.0
```

## 5. 手动重试

`Publish image` 工作流支持手动运行，但输入必须是已经存在的版本 Tag，例如 `v1.2.0-beta.1`。渠道由 Tag 自动识别，不能手工指定。

手动运行用于失败发布的重试。若精确版本镜像已经成功存在，工作流会拒绝覆盖；应直接使用已有镜像，而不是重建同一版本。如确实发布了损坏产物，应先调查原因并发布新的补丁/Beta 序号，不移动旧 Tag。

## 6. 回滚与核验

回滚只需恢复到之前的精确版本：

```bash
docker pull ghcr.io/<repository-owner>/oscraper:1.1.3
docker compose up -d
```

不要依赖移动 `latest` 来记录生产版本。每次发布完成后，Actions Summary 会记录渠道、精确镜像地址和 Manifest digest；也可以核验远程多架构 Manifest：

```bash
docker buildx imagetools inspect ghcr.io/<repository-owner>/oscraper:1.2.0
```

发布流程不会自动执行数据库回退。涉及版本回退时，先遵循 `docs/operations.md` 的备份和恢复步骤。
