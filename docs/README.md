# May Monitor Server 文档

当前教程基线为 Server／Agent／Web 0.1.0；配套 Status 2.2.2-may.17、Deploy 0.9.38。中文为主，[English overview](../README.en.md) 与 [English operations](OPERATIONS.en.md) 提供英文说明。

| 顺序 | 文档 |
| --- | --- |
| 1. 了解职责与平台 | [项目首页](../README.md) |
| 2. 安装、HTTPS、初始化、Agent 与维护 | [运维教程](OPERATIONS.zh-CN.md) |
| 3. 指标、保留、探测与规则 | [数据与告警](MONITORING.zh-CN.md) |
| 4. 启用外发 | [邮件配置与验收](EMAIL.zh-CN.md) |
| 5. 配套接入 | [只读接口](READ-ONLY-INTEGRATION.zh-CN.md)、[Deploy 生命周期](DEPLOY-LIFECYCLE.zh-CN.md) |
| 6. 开发和构建 | [Build](BUILD.md)、[SECURITY](../SECURITY.md) |

独立 watchdog 的完整单元与凭据续期见[Deploy 手册](https://github.com/mxh110708/may-vps-deploy/blob/main/docs/MONITOR-WATCHDOG.zh-CN.md)，公开主题见[May Status](https://github.com/mxh110708/may-monitor-theme-status)。

## 历史证据

- [0.1.0 稳定发行](../RELEASE-0.1.0.md)、[CHANGELOG](../CHANGELOG.md)。
- [2026-10-24 初始安全审查](SECURITY-REVIEW-2026-10-24.zh-CN.md)。
- [RC7](../RELEASE-0.1.0-rc.7.md)、[RC8](../RELEASE-0.1.0-rc.8.md)、[RC9](../RELEASE-0.1.0-rc.9.md)：以各候选当时范围为准。
- [M5 冻结长测](https://github.com/mxh110708/may-vps-deploy/blob/main/docs/MONITOR-M4-M5-2026-10-25.zh-CN.md)与[首阶段收尾](https://github.com/mxh110708/may-vps-deploy/blob/main/docs/MONITOR-CLOSEOUT-2026-10-27.zh-CN.md)：保留历史异常，不将 RC 长测改称稳定版新长测。

旧上游安装／自动发行材料、可选 jsruntime API 与底层 RPC 参考不构成默认监控能力的安装许可。当前正常 schema 维护仍保留，完成的旧产品迁移桥不再提供通用导入。
