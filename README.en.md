# May Monitor

A self-hosted monitoring system for host telemetry, tiered history, scheduled network probes and confirmed alerts. This repository contains the Server; Agent and Web are maintained separately, and May Status is an optional public status theme.

[简体中文](README.md) · [Releases](https://github.com/mxh110708/may-monitor-server/releases/latest) · [Operations](docs/OPERATIONS.en.md) · [Documentation](docs/README.md)

## Components and scope

The documentation baseline is Server/Agent/Web **0.1.0**, May Status **2.2.2-may.17** and Deploy **0.9.38**, checked on 2026-10-27. Debian 12/13 amd64 is the lifecycle acceptance target; a cross-compiled artifact is not native runtime acceptance.

- Server stores identities, metrics, history, probe tasks, alert policies and events.
- [Agent](https://github.com/mxh110708/may-monitor-agent) collects host data and runs configured probes.
- [Web](https://github.com/mxh110708/may-monitor-web) supplies the embedded default UI, setup and administration.
- [May Status](https://github.com/mxh110708/may-monitor-theme-status) displays public status; it is not an administration server.
- [Deploy](https://github.com/mxh110708/may-vps-deploy) manages component lifecycle and private archives from Windows.

Monitoring supports offline, latency, probe-failure-rate, CPU, memory and disk rules, with confirmation, recovery, cooldown, silence and bounded events. Optional TLS SMTP delivery requires a private sender configuration and explicit activation. A configured sender does not prove inbox delivery.

Remote control, terminal/file access, arbitrary SQL, general extensions, debug endpoints and Agent auto-update are disabled by default. Read [SECURITY.md](SECURITY.md) before changing any capability.

## Deployment

Use Deploy's managed lifecycle or follow the [operations guide](docs/OPERATIONS.en.md). Verify fixed release assets and checksums, create a dedicated account and private data directory, bind explicitly to loopback, configure HTTPS and compare installation identities before connecting agents.

The standalone Server default is `0.0.0.0:25774`; the managed installation explicitly overrides it with a loopback listener. Do not assume that downloading a May binary configures network exposure or a firewall. Historical upstream installers are disabled references, not current deployment instructions.

The administrator account is created during first-run setup. Its password is different from the VPS root password, Agent Token and scoped read credential. There is no public universal default password.

## Interfaces and maintenance

The [read-only endpoint](docs/READ-ONLY-INTEGRATION.zh-CN.md) requires one explicit node scope, token digest and expiry. Administrator keys and Agent tokens cannot replace it. Renewing or rotating the current grant affects all clients sharing that grant, including Deploy and an independent watchdog.

The Linux `watchdog` subcommand should run on a different host to observe a Server-host outage. It uses normal HTTPS verification and an expected installation UUID, and persists exclusively owned state. It does not replace per-node probes.

Read the [data and alert guide](docs/MONITORING.zh-CN.md), [email operations](docs/EMAIL.zh-CN.md), [lifecycle contract](docs/DEPLOY-LIFECYCLE.zh-CN.md) and [build instructions](docs/BUILD.md). Preserve component-scoped backups and current database schema procedures. The completed legacy migration bridges are not a general backup format.

## Evidence and attribution

The stable release promotes accepted candidate implementations. The frozen RC4/RC2 M5 soak retains its historical anomaly and capture-order repair; it is not a new 24-hour soak of 0.1.0. Reports distinguish fixture tests, real protocol checks, production observations and long-duration evidence.

May Monitor is derived from [Komari](https://github.com/komari-monitor/komari), with independently selected and reviewed updates. Retained Go module names and protocol fields are technical identities, not an endorsement of every upstream feature. See [LICENSE](LICENSE) and [CHANGELOG](CHANGELOG.md).
