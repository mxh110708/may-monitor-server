# May Monitor

自托管的服务器监控系统：采集主机指标，保存分层历史，执行指定节点的网络探测，并通过确认、恢复、冷却与静默规则管理告警。Server、Agent 和 Web 独立维护；May Status 提供可选的公开状态页。

[English](README.en.md) · [中文入口](README_zh-cn.md) · [发行版](https://github.com/mxh110708/may-monitor-server/releases/latest) · [安装与维护](docs/OPERATIONS.zh-CN.md) · [文档目录](docs/README.md)

## 组件和当前基线

| 组件 | 职责 | 文档基线 |
| --- | --- | --- |
| 本仓库 Server | 节点身份、指标存储、历史、探测任务、告警及 HTTP／RPC 接口 | 0.1.0 |
| [May Monitor Agent](https://github.com/mxh110708/may-monitor-agent) | 主机采集、连接与上报、执行已配置探测 | 0.1.0 |
| [May Monitor Web](https://github.com/mxh110708/may-monitor-web) | 默认前端、初始化、管理、历史和告警界面，构建后嵌入 Server | 0.1.0 |
| [May Status](https://github.com/mxh110708/may-monitor-theme-status) | 经审查、摘要固定的公开状态主题 | 2.2.2-may.17 |
| [May VPS Deploy](https://github.com/mxh110708/may-vps-deploy) | Windows 运维控制端，管理组件、私人归档、HTTPS 来源和只读接入 | 0.9.38 |

以上是 2026-10-27 文档核对时的配套版本；升级前仍要核对所选 Release、锁文件和兼容说明。正式生命周期验收目标为 Debian 12／13 amd64；ARM 等交叉编译附件不等于对应平台已完成运行验收。

## 功能与默认边界

- CPU、内存、磁盘、流量等主机指标，区分在线、首次等待、报告过期、无效值和服务不可达。
- 有界采集、分层指标历史与探测历史；延迟和探测失败率保留实际数据口径。
- 离线、延迟、探测失败率、CPU、内存及磁盘规则，支持连续确认、恢复条件、冷却、维护静默和事件记录。
- 可选邮件投递、本地回环 HTTP 接收器；邮件需私密 SMTP 配置和显式启用。配置就绪不等于已收到真实邮件。
- 独立鉴权、指定节点、带期限的只读快照；管理员、Agent 和读取凭据互不替代。
- 同一 Linux 程序提供独立 `watchdog` 子命令，可放到另一台主机观察 Server 失联。
- 静态主题按固定来源与摘要启用；后台、初始化和恢复继续使用内置 Web。

远程终端／命令／文件控制、任意 SQL、通用扩展、调试和 Agent 自动更新默认关闭。默认监控能力不需要开启它们。具体边界、依赖审查与已知限制见 [SECURITY.md](SECURITY.md)。

## 开始使用

推荐在 May VPS Deploy 的实例维护中安装固定 Server 和 Agent。部署流程建立专用账户、回环监听、配置归属和组件快照，再核对 HTTPS 的安装 UUID 与 Agent Token。详见[安装与维护](docs/OPERATIONS.zh-CN.md)及[Deploy 生命周期合同](docs/DEPLOY-LIFECYCLE.zh-CN.md)。

手动运维者应先核对发行附件、SHA-256、工作目录、专用账户和 HTTPS 入口，再启动首次初始化。独立二进制的默认监听为 `0.0.0.0:25774`，**不会因为使用 May 名称就自动只监听回环**；受管部署显式传入回环地址。不要直接运行旧上游安装脚本或将后台无保护暴露到公网。

管理员账户在首次初始化中创建；它不是 VPS root 密码、Agent Token 或只读令牌。初始化链接与归档均为私人材料。找不到密码时先核对自己的私人归档和受管恢复流程，不能从公开 README 查到通用默认密码。

## 阅读顺序

| 任务 | 文档 |
| --- | --- |
| 安装、HTTPS、初始化、Agent 和日常维护 | [运维教程](docs/OPERATIONS.zh-CN.md) / [English](docs/OPERATIONS.en.md) |
| 历史、探测、阈值、静默和通知 | [数据与告警](docs/MONITORING.zh-CN.md) |
| SMTP、重试与独立检查发信 | [邮件](docs/EMAIL.zh-CN.md) |
| Deploy／watchdog 读取授权 | [只读接口](docs/READ-ONLY-INTEGRATION.zh-CN.md) |
| 固定前端和源码构建 | [Build](docs/BUILD.md) |
| 旧轮次验收、已知缺口与变更 | [文档目录](docs/README.md)、[0.1.0 说明](RELEASE-0.1.0.md)、[CHANGELOG](CHANGELOG.md) |

## 证据与来源

0.1.0 是已验收候选实现的稳定发行。M5 的 24 小时长测使用冻结的 RC4／RC2 资产，保留了一次历史异常及后续采样顺序修正记录；它不是 0.1.0 的另一轮全新 24 小时验收。实际历史、资源和清理证据见[Deploy 收尾记录](https://github.com/mxh110708/may-vps-deploy/blob/main/docs/MONITOR-CLOSEOUT-2026-10-27.zh-CN.md)。

May Monitor 源自 [Komari](https://github.com/komari-monitor/komari)，独立选择、审查和适配上游变更，不自动跟随其发行。Go 模块、部分数据文件及协议字段中的 `komari` 是保留的技术名称；不代表通用扩展或旧迁移工具仍获支持。许可证与版权声明见 [LICENSE](LICENSE)。

反馈请附 Server／Agent／Web／主题版本、浏览器、发生时间、数据是否推进和脱敏错误；不要上传数据库、完整备份、节点 Token 或 SMTP 配置。
