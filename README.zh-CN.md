# OScraper

[English](README.md) · 简体中文

OScraper 是一个面向 OpenList 和本地媒体目录的自托管刮削工作台。它可以按需扫描电影、电视剧和动漫，通过 TMDB 完成匹配，在执行前展示全部变更计划，并在确认后安全地整理媒体文件和写入兼容的元数据。

OScraper 适合希望保留人工确认、操作边界和审计记录的用户。扫描和预览不会修改文件；正式执行遇到目标冲突时会停止，而不是覆盖已有媒体。

## 核心能力

- **支持 OpenList 和本地存储**：可以使用受控的 OpenList 目录，也可以把宿主机目录挂载到 `/media`。
- **支持电影、电视剧和动漫**：解析标题、年份、季集、动漫绝对集数以及 `{tmdbid-N}` 标记。
- **TMDB 辅助匹配**：可以按标题和年份搜索，也可以选择准确的 TMDB ID。
- **执行前完整预览**：查看目录创建、文件重命名、NFO、海报、背景图和分集图片计划。
- **安全写入**：重新检查目录指纹、拒绝过期预览，并且不会覆盖已存在的媒体路径。
- **可恢复作业**：通过持久化的有界 Worker 执行刮削，记录每项操作的检查点并支持失败重试。
- **媒体服务器元数据**：生成兼容 Kodi、Jellyfin 和 Emby 的电影、剧集和分集 NFO XML。
- **完整运行记录**：搜索作业历史、API 日志、应用日志和管理员审计日志，并支持 CSV 导出。
- **易用界面**：响应式明暗主题，支持英文和简体中文。
- **简单自托管**：单个镜像同时支持 `linux/amd64` 和 `linux/arm64`，使用 SQLite 存储。

## 工作流程

```text
连接存储 → 创建受控目标 → 扫描媒体候选
    → 确认 TMDB 匹配 → 检查不可变执行计划
    → 确认执行并监控持久化作业
```

扫描和预览阶段均为只读。只有明确确认有效计划后，OScraper 才会开始重命名媒体和写入元数据。

## 使用要求

- 安装了 Docker Compose v2 的 Docker Engine
- TMDB v3 API Key
- 以下至少一种媒体来源：
  - OpenList 账号及能够访问目标媒体目录的 Token；或
  - 可以挂载进容器的宿主机本地目录

使用 OpenList 时，建议创建只允许访问媒体库根目录的专用账号。若需要重命名和写入元数据，Token 必须具备列目录、创建目录、移动、重命名和上传权限。

## 使用 Docker Compose 部署

OScraper 镜像仅发布到 GitHub Container Registry：

```text
ghcr.io/hienao/oscraper
```

下面的示例固定到当前 Beta 版本。建议使用精确版本，以便明确控制升级和回退。

### 1. 准备部署目录

```bash
mkdir -p oscraper/media
cd oscraper
```

创建 `compose.yaml`：

```yaml
services:
  oscraper:
    image: ${OSCRAPER_IMAGE:-ghcr.io/hienao/oscraper:0.1.0-beta.1}
    container_name: oscraper
    ports:
      - "3113:3113"
    environment:
      APP_ENV: production
      JWT_SECRET: ${JWT_SECRET:?JWT_SECRET is required}
      CREDENTIAL_ENCRYPTION_KEY: ${CREDENTIAL_ENCRYPTION_KEY:?CREDENTIAL_ENCRYPTION_KEY is required}
      TZ: ${TZ:-UTC}
      SCRAPE_WORKERS: ${SCRAPE_WORKERS:-2}
      SCAN_WORKERS: ${SCAN_WORKERS:-1}
    volumes:
      - oscraper-data:/data
      - oscraper-cache:/cache
      - ${HOST_MEDIA_DIR:-./media}:/media
    restart: unless-stopped

volumes:
  oscraper-data:
  oscraper-cache:
```

### 2. 创建环境变量文件

生成两个互相独立的密钥：

```bash
openssl rand -hex 32
openssl rand -base64 32
```

创建 `.env`，把第一个结果写入 `JWT_SECRET`，第二个结果写入 `CREDENTIAL_ENCRYPTION_KEY`：

```dotenv
OSCRAPER_IMAGE=ghcr.io/hienao/oscraper:0.1.0-beta.1
JWT_SECRET=替换为第一个命令生成的值
CREDENTIAL_ENCRYPTION_KEY=替换为第二个命令生成的值
TZ=Asia/Shanghai

# 暴露给本地刮削目标的宿主机目录，在容器内对应 /media。
HOST_MEDIA_DIR=./media

# 可选的有界 Worker 数量，有效范围为 1-4。
SCRAPE_WORKERS=2
SCAN_WORKERS=1
```

请妥善保存并始终保持 `CREDENTIAL_ENCRYPTION_KEY` 不变。该密钥丢失或更换后，已经保存的 OpenList Token 和 TMDB Key 将无法解密。

如果 GHCR 包为私有状态，需要先登录：

```bash
echo "$GHCR_TOKEN" | docker login ghcr.io -u YOUR_GITHUB_USERNAME --password-stdin
```

Token 需要具有 `read:packages` 权限。

### 3. 启动 OScraper

```bash
docker compose pull
docker compose up -d
docker compose ps
```

打开 <http://localhost:3113>。健康的部署也会响应以下接口：

```text
http://localhost:3113/api/health/live
http://localhost:3113/api/health/ready
```

首次登录使用一次性账号密码 `admin/admin`。登录后 OScraper 会立即要求更换管理员用户名和密码，完成后才能继续使用。

## 首次配置

1. 打开**设置**，填写 TMDB v3 API Key，选择元数据语言和地区，保存后执行连接测试。
2. 使用 OpenList 时，打开**连接**，填写服务地址和 Token，设置账号根目录并验证连接。
3. 打开**刮削目标**，创建电影、电视剧或动漫目标：
   - 选择 OpenList 连接及其允许根目录下的目录；或
   - 选择**本地目录**及 `/media` 下的目录。
4. 只有希望 OScraper 整理现有目录和文件名时才启用媒体重命名。关闭重命名后仍然可以生成元数据。
5. 执行扫描，检查识别出的媒体候选，必要时修正 TMDB 匹配，并查看完整预览。
6. 确认每项重命名和生成文件都正确后再执行，并在**作业**页面监控进度。

第一次处理真实媒体时，请使用媒体库中可恢复的小型副本，不要直接从主媒体库开始。

## 存储与权限

| 挂载点 | 用途 | 持久化要求 |
| --- | --- | --- |
| `/data` | SQLite 数据库、迁移记录和持久化作业工作区 | 必须持久化，示例使用 `oscraper-data` |
| `/cache` | API/应用日志及临时缓存 | 建议持久化，示例使用 `oscraper-cache` |
| `/media` | 暴露给本地刮削目标的宿主机媒体 | 仅本地目标需要 |

容器使用非 root 用户运行。扫描要求 `HOST_MEDIA_DIR` 可读，重命名或写入元数据要求该目录可写。本地目标中的符号链接会被主动忽略或拒绝。

OScraper 仅支持单实例运行。应用使用 SQLite 和本地作业检查点，请勿让多个运行中的容器同时挂载同一个 `/data` 卷。

## 升级

在 [GHCR 镜像包](https://github.com/hienao/OScraper/pkgs/container/oscraper)中查看可用标签，备份持久化数据卷，然后把 `OSCRAPER_IMAGE` 修改为需要的精确版本并运行：

```bash
docker compose pull
docker compose up -d
docker compose ps
```

仅当部署环境需要自动跟随最新 Beta 时才使用 `ghcr.io/hienao/oscraper:beta`。正式版也会发布 `latest`，但生产部署仍建议固定精确版本。

## 故障排查

查看容器日志：

```bash
docker compose logs --tail=200 oscraper
```

常见检查项：

- **容器无法启动**：确认 `JWT_SECRET` 不少于 32 个字符，`CREDENTIAL_ENCRYPTION_KEY` 是原始 32 字节值或 32 字节值的 Base64 编码。
- **本地媒体不可用**：检查 `HOST_MEDIA_DIR`、Docker 文件共享配置及宿主机读写权限。
- **OpenList 测试失败**：检查服务地址、Token、账号根目录和所需权限。
- **TMDB 测试失败**：检查 API Key、元数据地区/语言、代理和出站网络。
- **预览已过期**：扫描后源目录发生了变化，需要重新扫描并生成预览。
- **作业报告目标冲突**：人工检查已经存在的目标并重新生成计划，OScraper 不会覆盖目标。
- **作业被中断**：检查当前存储状态，然后在**作业**页面从保存的检查点重试。

备份恢复、权限配置、灰度验证和故障处理详见[运维指南](docs/operations.md)。

## 当前限制

- 当前版本为按需刮削，不包含定时扫描。
- OScraper 用于整理已有媒体和写入元数据，不生成 STRM 文件。
- 作业完成后不会自动刷新 Kodi、Jellyfin 或 Emby 媒体库。
- 本地目标不能跟随符号链接，也不能跨文件系统移动媒体。
- 不支持多个实例共享同一个数据库。

## 相关文档

- [运维与恢复](docs/operations.md)
- [发布渠道和镜像标签](docs/release.md)
- [架构与设计](docs/design.md)
