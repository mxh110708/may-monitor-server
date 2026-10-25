# May Deploy 配套生命周期契约

监控支线 M3补充：Server RC3 源码支持 `MAY_MONITOR_INSTANCE_ID`，由受管生命周期安装／升级时生成并保存到私有环境及归属记录，常规升级、组件恢复和重启保持稳定。它不是硬件 ID 或授权令牌。已认证快照及 Agent 凭据核验返回 `server_instance_id`，供 Deploy 比对所选 SSH 实例与 HTTPS 来源；旧 Agent 可以只读核验，不需改写配置。该配套候选尚未发布。

日期：2026-10-24。Server 候选 `0.1.0-rc.2` 搭配已审查 Agent／Web `0.1.0-rc.1`，尚未发布。使用独立的 May 服务与数据库，与原 Komari 并存，按用户选定节点逐步迁移。

## 服务端新增接口

`POST /api/clients/credential-check` 使用已有的 Agent Token 身份验证。请求 JSON 只有 `token`，不把凭据放进 URL；返回 `schema_version: 1`、该 Agent 的 `uuid` 和 `server_version`，带 `Cache-Control: no-store`。

该接口不创建节点、不上报指标、不修改设备信息，不返回 Token 或其他节点信息。管理员会话／API Key、独立只读凭据和未认证请求不能借此转化为 Agent 身份。Deploy 在新 Agent 安装前核对该端点；HTTPS 使用正常证书校验，不接收重定向或 Cookie，只有回环隔离测试允许 HTTP。

独立只读指标接口的授权范围、期限及撤销契约见[只读接口](READ-ONLY-INTEGRATION.zh-CN.md)。两者使用各自凭据，不能互换。

## 受管资源与恢复

Deploy 为 Server 和 Agent 各自建立 `may-monitor-server`／`may-monitor-agent` 无登录用户、systemd 服务、`/etc` 配置和 `/var/lib` 私有数据目录。Server 默认只监听回环，Agent 关闭远程终端和自动更新。旧 Komari、Tunnel、代理、SSH 和防火墙保持原归属。

程序来源为固定 May 发行附件，先验证版本、总大小、SHA-256 和归档成员。升级保留现有连接和启停状态。一致性快照在选中服务暂停期间生成；恢复只处理同机器、同角色、同事务和摘要匹配的快照，恢复前保留当前数据。

控制端和远端使用事务身份核对确认丢失，不自动重放安装或升级；未知对象、文件／权限漂移、服务 drop-in、链接和身份不明账号阻止写入。账号创建确认不明时不凭同名自动删除。

首次安装的 operator-supplied `MAY_MONITOR_SETUP_TOKEN` 不再输出到服务日志。自动生成、由运维者手动启动的安装向导仍保持既有一次性接入方式；两种来源在创建控制器时固定，避免运行时环境变化导致错误输出。

## 在线状态修正

WebSocket 持有真实连接状态，HTTP POST 回退持有限期租约。WebSocket 上报及拉取不再刷新 POST 租约，WebSocket 接管时停止自己的旧 POST 计时器。真实 WS 关闭后不再被遗留租约继续显示在线，也不会由旧计时器向已恢复的 WS 错发离线通知。其他节点的租约不受影响；HTTP 回退仍保留原超时机制。

## 验证与迁移边界

本轮与 Deploy 一起在新建 QEMU/KVM Debian 12／13 临时磁盘上验证：实际安装与初始化、RC1→RC2 升级、RC1 Agent Token 核验和指标、独立只读查询、断线重连、一致性备份、取消回退、程序／数据库恢复和重启。旧监控替身的文件、数据和 PID 保持一致。

60 秒单节点上报和只读查询中，Agent RSS 峰值约 15–17 MiB，Server 约 40–53 MiB；这是隔离短时观测，不是生产长期性能保证或优化收益比较。Go 全回归、连接／只读接口／Agent／安装／鉴权包的竞争检测及 Deploy 核心和原生表单回归另有本地证据。

旧 Komari 历史、用户和告警不自动导入；新 HTTPS 发布路径、生产长期资源、真实 SSO、所有可选扩展和生产节点迁移仍需单独验收。本轮没有安装用户应用或替换生产服务。
