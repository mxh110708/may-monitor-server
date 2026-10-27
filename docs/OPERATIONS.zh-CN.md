# May Monitor 安装与维护

基线：Server／Agent／Web 0.1.0。正式生命周期验收面向 Debian 12／13 amd64。以下路径是受管布局或明确标注的手动示例，不含实际实例地址和凭据。当前静态主题为独立产品 May Status，不能代替后台或 Agent。

## 先确定管理方式

使用 May VPS Deploy 时，以实例维护为入口，让其建立专用用户、systemd、私有配置、固定程序和恢复点。不要同时在同一路径运行手动安装脚本。已有非受管服务先核对归属和备份，名称相同不能自动接管。

手动运维时，操作者负责账户、监听、TLS、日志、数据库、一致性备份及恢复。不要直接执行保留在仓库中的 `*.upstream-disabled` 脚本，也不要为使用当前主题而开启所有上游扩展。

## 组件和路径

| 受管位置 | 责任 |
| --- | --- |
| `may-monitor-server` systemd 服务和系统账户 | Server 生命周期，使用无登录专用账户。 |
| `/etc/may-monitor-server/` | 受保护的服务环境与归属材料；不是公开配置仓库。 |
| `/var/lib/may-monitor-server/` | Server 工作目录，数据库、配置和状态；专用账户可写、目录私有。 |
| `may-monitor-agent` 及其 `/etc`／`/var/lib` 目录 | Agent 配置、报告与统计状态，与 Server 分开。 |
| `/usr/local/share/may-monitor/themes/<lock-sha256>/` | root 管理的固定静态主题；服务账户不能原地修改。 |
| Deploy 私人归档 | SSH 信任、安装身份、加密凭据和同组件恢复材料。 |

底层可能保留 `komari.db`、`komari-theme.json` 等技术文件名。不能仅凭文件名把正常数据库当作旧版本残留删除。数据库实际路径由命令与配置决定，应从本机受管记录核对。

## 安装 Server 与首次初始化

1. 选择发行版本和架构，校验附件字节数、SHA-256、发行说明与来源提交。源码 ZIP、Web dist 和 Server 程序不是同一种附件。
2. 核对选定机器、SSH 主机密钥、已有进程、监听和数据归属。先保存可用的恢复入口与同组件快照。
3. 使用专用无登录账户及私有工作目录，显式指定回环监听和数据库。受管安装由 Deploy 完成；手动调试的命令结构如下，尖括号须先替换：

```bash
./may-monitor-server server --listen '127.0.0.1:<LOOPBACK_PORT>' --database '/var/lib/may-monitor-server/data/komari.db'
```

程序独立运行默认监听 `0.0.0.0:25774`，所以生产不能省略监听检查。以进程实际参数和 `ss` 的归属为准，不能从 README 推断当前已只监听回环。

4. 配置受信 HTTPS 入口。反向代理或 Tunnel 与 Server 是独立组件；保留证书检查和鉴权请求头。只有实际使用且经核对的代理地址／网段才可写入 `MAY_MONITOR_TRUSTED_PROXIES`，不使用任意来源通配。
5. 首次初始化使用私人安装链接中的 setup 令牌。也可由操作者私密提供 `MAY_MONITOR_SETUP_TOKEN`（32–256 字符），受管安装不会把该值输出到服务日志。不要把安装链接贴到公开问题中。
6. 在内置初始化页面创建管理员并保存私人凭据。账户创建结束后安装令牌失效；重新打开首页不能再次当作首次安装。
7. 核对 HTTPS 证书、域名、页面和安装 UUID。`MAY_MONITOR_INSTANCE_ID` 是稳定的随机安装身份，不是硬件证明或授权秘密。普通升级保持它，复制整个身份和数据库不构成不可复制保证。

## 安装 Agent

在目标 Server 创建节点并取得该节点专用 Token，每个安装单独绑定。Server 管理员密码、只读令牌和自动发现密钥不能替代节点 Token。

使用 Deploy 安装时，写入前通过凭据核验确认 Client UUID 和 Server 安装 UUID，再安装固定 Agent。手动方式见[Agent 教程](https://github.com/mxh110708/may-monitor-agent/blob/main/docs/OPERATIONS.zh-CN.md)。HTTPS 必须可信，远程控制和自动更新维持关闭。

验收检查节点身份、实际版本、连接状态、报告时间连续推进及历史落盘。网页打开、systemd active 或一次合成上报都不能证明真实 Agent 持续正常。WebSocket 连接与 HTTP 回退租约分开管理，断开和新鲜度不能相互替代。

## 开放公开状态页

首次初始化默认私人站点。确认哪些节点、字段和页面允许公开后，再显式调整站点可见性。隐藏后台链接不是服务器鉴权；节点名字、地址、账单和备注也应分别检查公开范围。

使用 May Status 时按[主题教程](https://github.com/mxh110708/may-monitor-theme-status/blob/main/docs/USER-GUIDE.zh-CN.md)校验 ZIP、固定锁摘要并启用。管理、初始化和恢复路由继续由内置 May Web 提供。主题代码属于受信代码，不是浏览器安全沙箱；普通主题设置不授予安装任意扩展的权限。

## 只读接入和独立检查

Deploy 和 watchdog 读取使用专用只读快照，限定 UUID 范围和到期时间；不接受管理员会话或管理员 API Key 降格使用。详见[只读合同](READ-ONLY-INTEGRATION.zh-CN.md)。当前只有一份服务端读取授权，所有共享调用方要同步轮换与续期。

独立 watchdog 使用同一发行的 Linux Server 二进制：

```bash
./may-monitor-server watchdog --config '/etc/may-monitor-watchdog/watchdog.json' --state '/var/lib/may-monitor-watchdog/state.json'
```

令牌由受保护环境 `MAY_MONITOR_WATCHDOG_READ_TOKEN` 提供，不能写进 URL 或命令参数。检查器先取得健康证据再累计失联，同一状态文件只能由一个进程持有。应放在不同主机；同机 PrivateNetwork 测试不能覆盖 Server 宿主机故障。完整单元、权限、确认次数、凭据续期与卸载见[Deploy 独立检查手册](https://github.com/mxh110708/may-vps-deploy/blob/main/docs/MONITOR-WATCHDOG.zh-CN.md)。

## 数据与告警

先在[数据与告警教程](MONITORING.zh-CN.md)确认探测方向、指标单位、窗口与恢复阈值，再启用规则。缺失、过期或无效指标保持未知；公开主题的颜色不是告警条件，也不会自动驱动 Route 切换。

邮件默认不因配置文件存在而发送。先准备私密 SMTP 配置、收件人和授权码，核对服务端状态，再明确启用邮件和规则。配置就绪只说明本机校验通过；真实外发与收件箱到达要另行确认。详情见[邮件教程](EMAIL.zh-CN.md)。

## 升级与备份

记录当前版本、来源锁、安装 UUID、启停状态、读授权到期、主题锁及可恢复快照。Server 先升级，再按对应说明更新 Agent；Web 通常随 Server 嵌入，主题独立固定。不能从源码分支临时拼接另一套前端替换正式附件。

同组件一致性备份应包含程序、受管配置、数据库及所需外部主题引用。SQLite 写入期间不能只复制主 DB 忽略 WAL。受管生命周期在一致性快照窗口暂停相应服务，确认后再恢复运行；不可把无关 Tunnel、代理或 SSH 纳入默认恢复。

更新先校验完整归档再解包，遇到链接、未知文件、归属漂移或未处理事务停止写入。取消和提交确认丢失先核对同一事务。恢复前保留当前数据，避免把升级后新指标无必要地覆盖回旧快照。

恢复后核对服务、实际版本、安装 UUID、HTTPS、节点凭据、历史、当前报告和读授权，全部一致才提交。旧 Komari 导入桥已退出当前产品流程，不能拿它当作通用 May 恢复方法；正常的 May schema 维护仍然保留。

## 日常检查与排错

| 现象 | 检查顺序 |
| --- | --- |
| 页面可见但没有新指标 | 节点是否被授权显示 → Agent Token／身份 → 报告年龄 → 写入与存储。 |
| Agent 重连失败 | HTTPS、证书、系统时间、地址族、反向代理 WS 支持与专用凭据；不关闭 TLS 绕过。 |
| 只读返回 401 | 摘要、格式、节点范围、到期及进程环境；外部环境文件修改后需要受管重启生效。 |
| 前端资源白屏 | 正确版本、真实 JS／CSS 内容与 MIME、404、主题锁；HTML 200 不是资源成功。 |
| 收不到邮件 | 渠道／规则是否启用 → SMTP 配置状态 → 事件与重试类别 → 服务商／垃圾箱；不把本地夹具成功当成外发成功。 |
| DB／WAL 变大 | 历史层级、保留窗口、写入／清理时序、checkpoint 与磁盘余量；单次 WAL 峰值不等于泄漏。 |
| 忘记管理员密码 | 私人归档、现有授权会话和同数据库的受管重置；不复用 root／Agent 密码。 |

源码保留 `chpasswd` 命令，但它会把新密码放在参数和输出中。不要在共享终端或日志采集环境直接照抄；先确认数据库、备份与日志隔离，并遵循当前受管恢复流程。不能通过删数据库解决忘记密码。

退役先备份并确认替代监测持续，再停用明确选中的组件。删除旧数据与备份另行决定保留范围；不要恢复已提交的旧切换脚本。
