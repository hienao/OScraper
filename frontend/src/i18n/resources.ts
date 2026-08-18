const en = {
  translation: {
    common: {
      productDescription: 'OpenList media scraping workbench',
      save: 'Save', cancel: 'Cancel', close: 'Close', edit: 'Edit', remove: 'Delete', test: 'Test', refresh: 'Refresh',
      enabled: 'Enabled', disabled: 'Disabled', healthy: 'Healthy', untested: 'Untested', failed: 'Failed', loading: 'Loading…',
    },
    language: { label: 'Language', auto: 'Auto', english: 'English', chinese: '简体中文' },
    theme: { dark: 'Use dark theme', light: 'Use light theme' },
    navigation: {
      aria: 'Application navigation', open: 'Open navigation', close: 'Close navigation', skip: 'Skip to content', logout: 'Log out',
      primary: 'Workspace', administration: 'Administration', overview: 'Overview', connections: 'OpenList connections', targets: 'Scrape targets', settings: 'Scraping settings', logs: 'Logs', profile: 'Profile', administrator: 'Administrator',
    },
    auth: {
      login: { title: 'Welcome back', description: 'Sign in to manage OpenList scraping', username: 'Username', password: 'Password', submit: 'Sign in', submitting: 'Signing in…', hint: 'First start: use admin / admin, then set permanent credentials.', failed: 'Sign-in failed' },
      setup: { title: 'Secure the administrator account', description: 'Replace the one-time admin/admin credentials before continuing.', username: 'Administrator username', password: 'New password', confirm: 'Confirm password', submit: 'Complete setup', mismatch: 'Passwords do not match', failed: 'Administrator setup failed' },
    },
    dashboard: {
      eyebrow: 'Scrape with confidence', title: 'Your OpenList media, organized and enriched', description: 'Connect OpenList, constrain a media root, and run a read-only candidate scan. TMDB matching and execution build on the saved fingerprint.',
      connections: 'Connections', connectionsDescription: 'Configured OpenList servers', jobs: 'Scrape jobs', jobsDescription: 'Job engine arrives in the next milestone', safety: 'Safety first', safetyDescription: 'Tokens are encrypted and never returned by the API', getStarted: 'Add your first connection',
    },
    connections: {
      title: 'OpenList connections', description: 'Connections are validated before saving. Tokens are encrypted and never shown again.', add: 'Add connection', empty: 'No OpenList connections yet.', emptyDescription: 'Add a server to begin browsing media directories.',
      name: 'Name', baseUrl: 'Base URL', token: 'Token', tokenOptional: 'New token (optional)', qps: 'File API QPS', qpm: 'File API QPM', account: 'Account', basePath: 'Account root', lastTest: 'Last test', status: 'Status', actions: 'Actions',
      createTitle: 'Add OpenList connection', editTitle: 'Edit OpenList connection', createDescription: 'Saving performs a live /api/me check.', editDescription: 'Changing the URL marks the connection untested until the next check.',
      saving: 'Saving…', testing: 'Testing…', saved: 'Connection saved', testPassed: 'Connection test passed', tokenRotated: 'Token rotated', deleted: 'Connection deleted',
      deleteConfirm: 'Delete connection “{{name}}”?', formError: 'Could not save the connection', testError: 'Connection test failed', deleteError: 'Could not delete the connection', placeholderName: 'Home media', placeholderUrl: 'http://openlist.local:5244',
    },
    targets: {
      title: 'Scrape targets', description: 'Constrain scraping to an explicit OpenList root and media type.', add: 'Add target', empty: 'No scrape targets yet.', emptyDescription: 'Create a target after configuring an OpenList connection.',
      name: 'Name', connection: 'OpenList connection', rootPath: 'Root path', libraryType: 'Media type', movie: 'Movie', tv: 'TV show', anime: 'Anime', rename: 'Allow media rename', renameWarning: 'Enables rename planning for this target. Execution will still require an explicit preview and confirmation.', enabled: 'Target enabled',
      createTitle: 'Add scrape target', editTitle: 'Edit scrape target', createDescription: 'The directory is read once before the target is saved.', browse: 'Browse directory', browserTitle: 'OpenList directory', currentPath: 'Current path', up: 'Up one level', noEntries: 'This directory is empty.', file: 'File', directory: 'Directory',
      scan: 'Scan media', scanning: 'Scanning…', scanTitle: 'Media scan', scanDescription: 'Read-only discovery; no OpenList files are modified.', scanningDescription: 'Reading the target recursively and identifying media candidates…', scanStatus: 'Status', scanSucceeded: 'Completed', candidates: 'Candidates', videoFiles: 'Video files', noCandidates: 'No media candidates were found.', ready: 'Ready', needs_review: 'Needs review', confidence: '{{value}}% confidence', videoCount: '{{count}} video', videoCount_other: '{{count}} videos', scanError: 'Could not scan the target',
      tmdbPreview: 'TMDB preview', searchTitle: 'Search title', searchYear: 'Year', searchTMDB: 'Search TMDB', searching: 'Searching…', noTMDBResults: 'TMDB returned no matching results.', noOverview: 'No overview is available.', selectMatch: 'Use this match', matchError: 'Could not search TMDB', previewError: 'Could not create the scrape preview', previewReady: 'Ready for planning', previewBlocked: 'Blocked', readOnly: 'Read only', renamePlan: 'Rename plan', generatedFiles: 'Generated metadata', previewWarnings: 'Review required', previewExpires: 'This immutable preview expires at {{value}}.',
      warnings: { rename_disabled: 'Media rename is disabled for this target.', year_missing: 'TMDB did not provide a release year; execution will remain blocked.', episode_file_plan_pending: 'Episode and season rename expansion will be completed before execution is enabled.', conflict_check_pending: 'Destination conflicts will be checked again against a fresh OpenList listing before execution.' },
      saving: 'Saving…', saved: 'Target saved', deleted: 'Target deleted', deleteConfirm: 'Delete target “{{name}}”?', formError: 'Could not save the target', deleteError: 'Could not delete the target', browserError: 'Could not read the directory', placeholderName: 'Movies', placeholderPath: '/media/movies',
    },
    settings: {
      title: 'Scraping settings', description: 'Configure TMDB matching. Secrets are encrypted and never returned by the API.', apiKey: 'TMDB API key', apiKeySaved: 'An encrypted key is saved. Leave blank to keep it unchanged.', apiKeyMissing: 'No API key is configured.', apiKeyPlaceholder: 'Enter a TMDB v3 API key', baseUrl: 'TMDB API base URL', imageBaseUrl: 'TMDB image base URL', language: 'Metadata language', languageHint: 'Use a TMDB language code such as zh-CN or en-US.', region: 'Region', regionHint: 'Optional ISO country code such as CN or US.', posterSize: 'Poster size', backdropSize: 'Backdrop size', timeout: 'Request timeout (seconds)', saveError: 'Could not save scraping settings', saved: 'Scraping settings saved', test: 'Test saved configuration', testing: 'Testing…', testPassed: 'TMDB connection test passed', testError: 'TMDB connection test failed', saving: 'Saving…',
    },
    profile: { title: 'Profile', description: 'Current authenticated account', username: 'Username', role: 'Role' },
    logs: { description: 'API, application and audit log endpoints are ready. Searchable log tables will arrive with the scraping job UI.' },
    errors: {
      invalidResponse: 'The server returned an invalid response.', requestFailed: 'Request failed.',
      codes: {
        auth: { invalid_credentials: 'Invalid username or password.', token_missing: 'Please sign in.', token_invalid: 'Your session is invalid or expired.', token_revoked: 'Your session has ended. Please sign in again.', setup_required: 'Complete administrator setup first.', admin_required: 'Administrator permission is required.' },
        connection: { not_found: 'OpenList connection not found.', invalid_id: 'Connection ID is invalid.' },
        target: { not_found: 'Scrape target not found.', invalid_id: 'Target ID is invalid.', invalid_path: 'OpenList path is invalid.', invalid_library_type: 'The media type is invalid.', path_outside_root: 'The path is outside the scrape target root.', path_outside_account: 'The path is outside the OpenList account root.', connection_disabled: 'The OpenList connection is disabled.' },
        scan: { not_found: 'Directory scan not found.', invalid_id: 'Scan ID is invalid.', target_disabled: 'The scrape target is disabled.', already_running: 'A scan is already running for this target.', too_large: 'The directory contains too many entries for one safe scan.', too_deep: 'The directory exceeds the safe scan depth.', path_outside_candidate: 'OpenList returned an entry outside the candidate root.', canceled: 'The directory scan was canceled.' },
        candidate: { not_found: 'Media candidate not found.' },
        preview: { not_found: 'Scrape preview not found.', invalid_id: 'Preview ID is invalid.', invalid_snapshot: 'The stored preview snapshot is invalid.' },
        tmdb: { not_configured: 'Configure a TMDB API key first.', invalid_query: 'Enter a search title.', invalid_id: 'TMDB ID is invalid.', invalid_url: 'TMDB API URL is invalid.', invalid_image_url: 'TMDB image URL is invalid.', invalid_language: 'TMDB language code is invalid.', invalid_region: 'TMDB region code is invalid.', invalid_image_size: 'TMDB image size is invalid.', authentication_failed: 'TMDB API key is invalid.', connection_failed: 'Could not connect to TMDB.', timeout: 'TMDB request timed out.', rate_limited: 'TMDB rate limit was exceeded.', not_found: 'TMDB item was not found.', no_results: 'TMDB returned no matching results.', invalid_response: 'TMDB returned an invalid response.', http_error: 'TMDB returned an HTTP error.' },
        openlist: { invalid_url: 'OpenList URL is invalid.', invalid_scheme: 'Use an HTTP or HTTPS OpenList URL.', dns_failed: 'OpenList host could not be resolved.', blocked_address: 'This network address is blocked.', connection_failed: 'Could not connect to OpenList.', authentication_failed: 'The OpenList token is invalid or lacks permission.', account_disabled: 'The OpenList account is disabled.', invalid_response: 'OpenList returned an invalid response.', api_error: 'OpenList rejected the request.', http_error: 'OpenList returned an HTTP error.' },
      },
    },
  },
}

const zhCN = {
  translation: {
    common: {
      productDescription: 'OpenList 媒体目录刮削工作台',
      save: '保存', cancel: '取消', close: '关闭', edit: '编辑', remove: '删除', test: '测试', refresh: '刷新',
      enabled: '已启用', disabled: '已停用', healthy: '正常', untested: '未测试', failed: '失败', loading: '加载中…',
    },
    language: { label: '语言', auto: '自动', english: 'English', chinese: '简体中文' },
    theme: { dark: '切换到深色主题', light: '切换到浅色主题' },
    navigation: {
      aria: '应用导航', open: '打开导航', close: '关闭导航', skip: '跳到主要内容', logout: '退出登录',
      primary: '工作台', administration: '管理', overview: '概览', connections: 'OpenList 连接', targets: '刮削目标', settings: '刮削设置', logs: '日志', profile: '个人资料', administrator: '管理员',
    },
    auth: {
      login: { title: '欢迎回来', description: '登录后管理 OpenList 刮削', username: '用户名', password: '密码', submit: '登录', submitting: '登录中…', hint: '首次启动请使用 admin / admin，随后设置正式凭据。', failed: '登录失败' },
      setup: { title: '保护管理员账户', description: '继续使用前，请替换一次性 admin/admin 凭据。', username: '管理员用户名', password: '新密码', confirm: '确认密码', submit: '完成初始化', mismatch: '两次输入的密码不一致', failed: '管理员初始化失败' },
    },
    dashboard: {
      eyebrow: '安心刮削', title: '整理并丰富你的 OpenList 媒体', description: '连接 OpenList、约束媒体根目录，再执行只读候选扫描；后续 TMDB 匹配与执行会基于已保存的目录指纹。',
      connections: '连接', connectionsDescription: '已配置的 OpenList 服务器', jobs: '刮削作业', jobsDescription: '作业引擎将在下一阶段接入', safety: '安全优先', safetyDescription: 'Token 加密保存且 API 永不返回明文', getStarted: '添加第一个连接',
    },
    connections: {
      title: 'OpenList 连接', description: '连接保存前会自动验证，Token 加密保存且不会再次显示。', add: '添加连接', empty: '还没有 OpenList 连接。', emptyDescription: '添加服务器后即可开始浏览媒体目录。',
      name: '名称', baseUrl: 'Base URL', token: 'Token', tokenOptional: '新 Token（可选）', qps: '文件 API QPS', qpm: '文件 API QPM', account: '账号', basePath: '账号根目录', lastTest: '最近测试', status: '状态', actions: '操作',
      createTitle: '添加 OpenList 连接', editTitle: '编辑 OpenList 连接', createDescription: '保存时会实时调用 /api/me 验证。', editDescription: '修改 URL 后会标记为未测试，直到下次检查。',
      saving: '保存中…', testing: '测试中…', saved: '连接已保存', testPassed: '连接测试通过', tokenRotated: 'Token 已更新', deleted: '连接已删除',
      deleteConfirm: '确定删除连接“{{name}}”吗？', formError: '保存连接失败', testError: '连接测试失败', deleteError: '删除连接失败', placeholderName: '家庭媒体', placeholderUrl: 'http://openlist.local:5244',
    },
    targets: {
      title: '刮削目标', description: '用明确的 OpenList 根目录和媒体类型约束刮削范围。', add: '添加目标', empty: '还没有刮削目标。', emptyDescription: '配置 OpenList 连接后创建一个刮削目标。',
      name: '名称', connection: 'OpenList 连接', rootPath: '根目录', libraryType: '媒体类型', movie: '电影', tv: '电视剧', anime: '动画', rename: '允许媒体重命名', renameWarning: '为该目标启用重命名计划；实际执行仍需经过明确预览和确认。', enabled: '启用目标',
      createTitle: '添加刮削目标', editTitle: '编辑刮削目标', createDescription: '保存前会读取一次目标目录进行验证。', browse: '浏览目录', browserTitle: 'OpenList 目录', currentPath: '当前路径', up: '返回上一级', noEntries: '该目录为空。', file: '文件', directory: '目录',
      scan: '扫描媒体', scanning: '扫描中…', scanTitle: '媒体扫描', scanDescription: '仅进行只读发现，不会修改 OpenList 文件。', scanningDescription: '正在递归读取目标目录并识别媒体候选…', scanStatus: '状态', scanSucceeded: '已完成', candidates: '候选', videoFiles: '视频文件', noCandidates: '未发现媒体候选。', ready: '可预览', needs_review: '需要检查', confidence: '置信度 {{value}}%', videoCount: '{{count}} 个视频', scanError: '扫描目标失败',
      tmdbPreview: 'TMDB 预览', searchTitle: '搜索标题', searchYear: '年份', searchTMDB: '搜索 TMDB', searching: '搜索中…', noTMDBResults: 'TMDB 未返回匹配结果。', noOverview: '暂无简介。', selectMatch: '使用该匹配', matchError: '搜索 TMDB 失败', previewError: '创建刮削预览失败', previewReady: '可进入计划', previewBlocked: '暂不可执行', readOnly: '只读', renamePlan: '重命名计划', generatedFiles: '将生成的元数据', previewWarnings: '需要确认', previewExpires: '该不可变预览将在 {{value}} 过期。',
      warnings: { rename_disabled: '该目标未启用媒体重命名。', year_missing: 'TMDB 未提供发行年份，执行将保持禁用。', episode_file_plan_pending: '季目录和剧集文件的完整重命名展开将在开放执行前完成。', conflict_check_pending: '执行前会重新读取 OpenList，并再次检查目标路径冲突。' },
      saving: '保存中…', saved: '目标已保存', deleted: '目标已删除', deleteConfirm: '确定删除目标“{{name}}”吗？', formError: '保存目标失败', deleteError: '删除目标失败', browserError: '读取目录失败', placeholderName: '电影库', placeholderPath: '/media/movies',
    },
    settings: {
      title: '刮削设置', description: '配置 TMDB 匹配；密钥加密保存且 API 永不返回明文。', apiKey: 'TMDB API Key', apiKeySaved: '已保存加密密钥，留空可保持不变。', apiKeyMissing: '尚未配置 API Key。', apiKeyPlaceholder: '输入 TMDB v3 API Key', baseUrl: 'TMDB API 基础地址', imageBaseUrl: 'TMDB 图片基础地址', language: '元数据语言', languageHint: '使用 zh-CN、en-US 等 TMDB 语言代码。', region: '地区', regionHint: '可选 ISO 国家代码，例如 CN 或 US。', posterSize: '海报尺寸', backdropSize: '背景图尺寸', timeout: '请求超时（秒）', saveError: '保存刮削设置失败', saved: '刮削设置已保存', test: '测试已保存配置', testing: '测试中…', testPassed: 'TMDB 连接测试通过', testError: 'TMDB 连接测试失败', saving: '保存中…',
    },
    profile: { title: '个人资料', description: '当前登录账户', username: '用户名', role: '角色' },
    logs: { description: 'API、业务和审计日志接口已经可用；可搜索日志表格将与刮削作业 UI 一起接入。' },
    errors: {
      invalidResponse: '服务端返回了无效响应。', requestFailed: '请求失败。',
      codes: {
        auth: { invalid_credentials: '用户名或密码错误。', token_missing: '请先登录。', token_invalid: '会话无效或已过期。', token_revoked: '会话已失效，请重新登录。', setup_required: '请先完成管理员初始化。', admin_required: '需要管理员权限。' },
        connection: { not_found: 'OpenList 连接不存在。', invalid_id: '连接 ID 无效。' },
        target: { not_found: '刮削目标不存在。', invalid_id: '目标 ID 无效。', invalid_path: 'OpenList 路径无效。', invalid_library_type: '媒体类型无效。', path_outside_root: '路径超出了刮削目标根目录。', path_outside_account: '路径超出了 OpenList 账号根目录。', connection_disabled: 'OpenList 连接已停用。' },
        scan: { not_found: '目录扫描不存在。', invalid_id: '扫描 ID 无效。', target_disabled: '刮削目标已停用。', already_running: '该目标已有扫描正在运行。', too_large: '目录条目过多，超出了单次安全扫描上限。', too_deep: '目录层级超出了安全扫描深度。', path_outside_candidate: 'OpenList 返回了候选根目录以外的条目。', canceled: '目录扫描已取消。' },
        candidate: { not_found: '媒体候选不存在。' },
        preview: { not_found: '刮削预览不存在。', invalid_id: '预览 ID 无效。', invalid_snapshot: '保存的预览快照无效。' },
        tmdb: { not_configured: '请先配置 TMDB API Key。', invalid_query: '请输入搜索标题。', invalid_id: 'TMDB ID 无效。', invalid_url: 'TMDB API 地址无效。', invalid_image_url: 'TMDB 图片地址无效。', invalid_language: 'TMDB 语言代码无效。', invalid_region: 'TMDB 地区代码无效。', invalid_image_size: 'TMDB 图片尺寸无效。', authentication_failed: 'TMDB API Key 无效。', connection_failed: '无法连接 TMDB。', timeout: 'TMDB 请求超时。', rate_limited: '已达到 TMDB 频率限制。', not_found: 'TMDB 条目不存在。', no_results: 'TMDB 未返回匹配结果。', invalid_response: 'TMDB 返回了无效响应。', http_error: 'TMDB 返回 HTTP 错误。' },
        openlist: { invalid_url: 'OpenList 地址无效。', invalid_scheme: 'OpenList 地址必须使用 HTTP 或 HTTPS。', dns_failed: '无法解析 OpenList 主机。', blocked_address: '该网络地址已被安全策略阻止。', connection_failed: '无法连接 OpenList。', authentication_failed: 'OpenList Token 无效或权限不足。', account_disabled: 'OpenList 账号已被禁用。', invalid_response: 'OpenList 返回了无效响应。', api_error: 'OpenList 拒绝了请求。', http_error: 'OpenList 返回 HTTP 错误。' },
      },
    },
  },
}

export const resources = { en, 'zh-CN': zhCN } as const
