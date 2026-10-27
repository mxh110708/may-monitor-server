# Deploy 配套生命周期合同

适用：May Monitor Server／Agent 0.1.0 与 May VPS Deploy 0.9.38。这里说明当前接口、身份和恢复合同；早期 RC 的临时系统测试保存在[文档目录](README.md)所列历史记录，不再作为「尚未发布」的安装状态。

## 身份与凭据

`MAY_MONITOR_INSTANCE_ID` 由受管安装生成并保存在私有环境和归属记录；正常升级、重启、组件恢复保持稳定。它是安装来源标签，不是硬件 ID、不可复制证明或授权令牌。缺失时普通指标仍可用，但无法通过需要 UUID 的来源比对。

已经认证的快照及 Agent 凭据核验返回 `server_instance_id`。Deploy 同时核对 SSH 选定实例和 HTTPS 来源，验证证书，不接受重定向或 Cookie 作为凭据替代。

`POST /api/clients/credential-check` 使用已有 Agent Token，请求 JSON 只含 `token`。返回 `schema_version: 1`、该节点 `uuid`、`server_version` 及已配置的安装身份，使用 `Cache-Control: no-store`。接口不注册节点、不上报合成指标、不修改机器信息、不返回 Token 或其他节点数据。

管理员、独立读取、未认证身份不能借此变成 Agent。只读指标授权见[接口合同](READ-ONLY-INTEGRATION.zh-CN.md)，不能与节点 Token 混用。

## 受管对象

Server 与 Agent 使用独立无登录账户、systemd、`/etc` 配置、`/var/lib` 数据和固定 May 资产。Server 显式回环监听；Agent 关闭远程控制与自动更新。Tunnel、代理、SSH 与旧备份各有自己的归属，不自动扩大当前事务范围。

安装和升级先核对版本、大小、SHA-256、归档全部成员与受管文件；发现链接、归属漂移、未知账号／单元或未处理事务时停止写入。初始化提供的 `MAY_MONITOR_SETUP_TOKEN` 不输出到服务日志，原始凭据只存受保护材料。

## 备份与恢复

快照按选定组件暂停写入，记录程序、配置、数据、摘要、版本、身份和原运行状态。恢复仅接受同机器、角色、事务及摘要匹配的材料，恢复前保存当前数据。未写入的升级恢复不应倒灌旧数据库覆盖新指标。

确认丢失时先只读核对同一事务身份和当前状态，不自动重放安装／升级。恢复完成需再验服务、安装 UUID、TLS、Agent 凭据与连续新报告。凭据被撤销后恢复旧文件不能算连接恢复。

旧 Komari 专用迁移导入桥已退出当前维护流程；当前 May 备份、schema 维护和组件恢复保留。历史资产不得替换为较新源码后继续声称是原长测。

## 在线与新鲜度

WebSocket 持有真实连接状态，HTTP POST 回退持有限期租约。WS 报告不延长 POST 租约，接管时停止对应旧计时器；关闭真实 WS 后不靠旧租约继续显示在线。在线与报告新鲜度分别返回和评估，采样时刻与快照时刻必须一致解释。

## 验证范围

本合同已有临时 Debian 12／13 的真实生命周期、对应接口／连接竞争检测及受管生产迁移记录，具体版本和缺口见[Deploy 文档](https://github.com/mxh110708/may-vps-deploy/blob/main/docs/README.md)。这些证据不自动覆盖未验收架构、上游可选扩展、所有 SSO 或新版本的长期性能；稳定 0.1.0 也未因此新增一轮 24 小时长测。
