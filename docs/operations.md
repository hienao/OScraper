# OScraper 运维与灰度指南

## 1. 上线前准备

从旧名称升级时，默认 SQLite 文件 `openlist-scraper.db` 和浏览器存储键仍保留原值作为兼容标识，请勿仅因应用更名而手工改名或删除。

1. 从 `.env.example` 复制 `.env`，生成独立的 `JWT_SECRET` 与 32 字节 `CREDENTIAL_ENCRYPTION_KEY`。加密密钥一旦更换，已有 OpenList Token 和 TMDB Key 将无法解密。
2. 应用仅支持 SQLite 和单实例部署；不要让多个应用容器同时挂载或访问同一个 SQLite 文件。
3. `/data` 保存业务数据库和作业工作区，`/cache` 保存 API/应用日志。日志页配置的保留期会同时清理 API、应用和审计日志；审计日志位于业务数据库。
4. OpenList Token 至少需要目标目录的读取、创建目录、移动、重命名和上传权限。建议使用只覆盖媒体库根目录的专用账号。
5. Linux/NAS 部署请执行 `id 运行用户`，将 UID 和主 GID 配置为 `PUID`、`PGID`。若媒体目录由另一个组授权，再将该组 ID 配置为 `MEDIA_GID`。容器只会自动初始化 `/data`、`/cache` 的所有权，不会修改 `/media`。
6. 本地刮削通过 `HOST_MEDIA_DIR` 挂载到容器 `/media`；扫描需要运行 UID 及其主组/附加组具备读取权限，重命名和元数据写入还需要写入权限。本地目标不跟随符号链接。

关键参数：

| 参数 | 默认值 | 说明 |
| --- | ---: | --- |
| `PUID` | 1000 | Nginx 与 OScraper 的运行用户 ID，必须大于 0 |
| `PGID` | 1000 | Nginx 与 OScraper 的运行组 ID，必须大于 0 |
| `MEDIA_GID` | 空 | 可选的媒体目录附加组 ID；媒体目录所属组与 `PGID` 不同时使用 |
| `UMASK` | 002 | 新建文件和目录的权限掩码，可使用三位或四位八进制格式 |
| `SCRAPE_WORKERS` | 2 | 并发 Worker，允许 1–4；同一 OpenList 连接的写操作仍串行 |
| `SCRAPE_QUEUE_SIZE` | 100 | 等待与运行作业的有界容量 |
| `SCAN_WORKERS` | 1 | 目录扫描并发数，允许 1–4；与写作业 Worker 隔离 |
| `SCAN_QUEUE_SIZE` | 20 | 等待与运行扫描的有界容量 |
| `MAX_IMAGE_BYTES` | 20971520 | 单张图片上限，允许 1–100 MiB |
| `JOB_RETENTION_DAYS` | 7 | 首次运行时作业记录及遗留作业工作区的默认保留天数，允许 1–30；之后可在作业页修改记录保留期（不影响工作区保留期） |
| `LOG_RETENTION_DAYS` | 7 | 首次运行时 API、应用和审计日志的默认保留天数，允许 1–30；之后可在日志页修改 |
| `DATA_RETENTION_DAYS` | 30 | 无引用候选项及扫描记录保留天数；预览仍按自身过期时间清理 |

本地目录示例：

```env
HOST_MEDIA_DIR=/mnt/nas/media
PUID=1026
PGID=100
MEDIA_GID=101
UMASK=002
```

上面的 UID/GID 仅为示例，请以宿主机 `id 运行用户` 的输出为准。容器内会看到 `/media/movies`、`/media/tv` 等子目录。不同本地刮削目标不能使用互相包含的根目录；本地写作业全局串行，且不支持跨文件系统移动。

## 2. 备份与恢复

执行备份前停止应用，避免复制到不一致的 SQLite/WAL 状态：

```bash
docker compose stop app
tar -czf oscraper-backup-$(date +%Y%m%d-%H%M%S).tar.gz runtime/data .env
docker compose start app
```

恢复时停止应用，把备份中的 `runtime/data` 和原始 `.env`（尤其是加密密钥）恢复到同一路径，再启动应用。不要只恢复数据库而丢弃失败作业仍引用的 `/data/work/jobs`。

恢复后先访问 `/api/health/ready`（就绪）和 `/api/health/live`（存活），再检查作业列表中是否有 `job.interrupted`，这些作业应由管理员确认后从检查点重试。健康报告同时包含 Job/扫描队列、日志丢弃计数、数据清理状态和本地挂载状态。

## 3. 升级与回退

1. 备份业务数据和密钥。
2. 拉取或构建新镜像，先运行 `docker compose config` 检查配置。
3. `docker compose up -d --build app`；启动时会按 `schema_migrations` 自动应用版本化迁移。
4. 检查健康接口、登录、连接测试、日志页和作业页。
5. 如果升级后需要回退，停止新版本，恢复升级前的数据备份和旧镜像。不要用旧二进制直接打开已经升级且未恢复的数据库。

## 4. 真实 OpenList 小规模灰度

请使用可恢复的测试副本或单独的小型媒体目录，不要第一次就在主媒体库执行。

- 电影：放入一个视频、一个同名字幕；再加入同标题年份的 2160p/1080p 两个版本，确认扫描后只形成一个电影候选，版本标签可编辑，字幕跟随对应版本，目标使用 `电影名 - 版本` 且只生成一个 `movie.nfo`。
- 电视剧：放入两集和一个字幕；确认 `Season 01`、`S01E01/S01E02`、show NFO、分集 NFO 与缩略图路径。
- 冲突保护：预览完成后手工创建一个目标同名文件，提交应被阻断或执行应以 `job.target_exists` 停止，现有文件不得改变。
- 断点恢复：在元数据上传阶段停止容器；重启后作业应显示 `job.interrupted`，点击重试后只继续未完成操作。
- 幂等：使用相同 `Idempotency-Key` 重复提交同一预览，应返回同一作业。
- 权限与脱敏：用权限不足 Token 验证错误可见但 Token 不出现在 API、应用或审计日志；普通未认证请求必须返回 401。
- 兼容性：在 Kodi/Jellyfin/Emby 中刷新该测试库，确认电影、剧集标题、简介、季集号和图片可读取。

灰度完成后从页面导出三类日志保存验收记录。任何路径或 TMDB 匹配不符合预期时停止扩大范围，重新扫描并生成新预览；不要复用过期或指纹不一致的预览。

## 5. 故障处理

- `preview.stale`：OpenList 目录在扫描/预览后变化，重新扫描。
- `job.target_exists`：目标路径已被占用；人工检查后重新规划，应用不会覆盖媒体。
- `job.interrupted`：进程退出导致；确认 OpenList 当前状态后从作业页重试。
- `job.queue_full`：等待现有作业结束，或在评估 OpenList 限流后提高队列；提高 Worker 不会绕过单连接串行保护。
- `scan.queue_full`：等待现有目录扫描结束，或调整独立的 `SCAN_QUEUE_SIZE`；扫描任务已持久化，进程重启后会继续处理未完成任务。
- `job.invalid_image_type` / `job.image_too_large`：检查 TMDB 图片代理响应和 `MAX_IMAGE_BYTES`。
- SQLite `busy/locked`：确认只有一个应用实例访问文件，并确认挂载存储支持文件锁；本应用不支持多实例共享数据库。
- `local.not_mounted` / `local.permission_denied`：检查 `HOST_MEDIA_DIR` 是否正确挂载，并确认状态接口展示的 UID/组与宿主机目录权限匹配。通常设置 `PUID`/`PGID` 即可；媒体目录使用其他共享组时设置 `MEDIA_GID`。容器不会自动修改 `/media` 权限。
- `local.cross_device_move`：源和目标落在不同文件系统；首版不会自动复制后删除，请调整挂载或目标目录。

应用收到 SIGTERM 后会停止接收新 HTTP 请求并等待作业退出，最长 10 秒。尚未安全结束的作业会在下次启动时标记为可重试。
