# Installation and operations

Baseline: Server/Agent/Web 0.1.0, with Debian 12/13 amd64 lifecycle acceptance. The [Chinese manual](OPERATIONS.zh-CN.md) includes detailed paths and troubleshooting.

## Install and verify

1. Select the exact Release and architecture. Verify its size, SHA-256 and source manifest. A Web bundle or source archive is not the Server executable.
2. Prefer the [Deploy](https://github.com/mxh110708/may-vps-deploy) managed lifecycle. It creates dedicated non-login accounts, private configuration, explicit loopback listeners and component-scoped snapshots. Do not mix manual writes with its transaction ownership.
3. For manual operation, provide your own account, private working directory, service unit, backup and HTTPS boundary. Explicitly pass the listener and database. The syntax below requires replacing the port placeholder:

```bash
./may-monitor-server server --listen '127.0.0.1:<LOOPBACK_PORT>' --database '/var/lib/may-monitor-server/data/komari.db'
```

The standalone default is `0.0.0.0:25774`. A managed loopback configuration is not a universal binary default. Verify the real listener and process ownership before exposing an HTTPS entry point.

4. Configure a trusted HTTPS reverse proxy or tunnel. Preserve authentication headers and normal certificate checks. Only explicitly trusted proxy addresses/CIDRs belong in `MAY_MONITOR_TRUSTED_PROXIES`.
5. Use the private first-run setup link to create the administrator. An operator-supplied `MAY_MONITOR_SETUP_TOKEN` must contain 32–256 characters and remain private. It is different from the administrator password and Agent tokens, and becomes unusable when setup completes.
6. Preserve a stable random `MAY_MONITOR_INSTANCE_ID`. Compare the installation UUID from the SSH-managed instance and authenticated HTTPS response. It is an origin label, not hardware attestation.
7. Create one node and token per Agent. Verify its Client UUID, Server UUID, advancing report time and persisted history. A running process or rendered page is insufficient.

Remote control, arbitrary SQL, extensions, debug capabilities and Agent auto-update remain off by default. Do not enable them to obtain ordinary monitoring or a reviewed static theme.

## Data, public display and read access

First-run setup defaults to a private site. Review node and field visibility before enabling public status. Hiding an administration link is not access control. May Status is optional; administration, setup and recovery remain on the embedded Web UI.

Missing, stale, invalid and unauthorized metrics remain unknown. Host-to-target ICMP, TCP and HTTP probes measure different paths; none automatically measures the user's full proxy session. See [metrics and alerts](MONITORING.zh-CN.md).

The [scoped read contract](READ-ONLY-INTEGRATION.zh-CN.md) uses one token digest, explicit node scope and expiry. There is currently one grant; every client sharing it must be updated on rotation, including Deploy and watchdog. Never substitute an administrator API key.

The Linux Server binary also provides:

```bash
./may-monitor-server watchdog --config '/etc/may-monitor-watchdog/watchdog.json' --state '/var/lib/may-monitor-watchdog/state.json'
```

Supply `MAY_MONITOR_WATCHDOG_READ_TOKEN` through a protected service environment. Run it on another host to cover a Server-host outage. A single state file has exclusive process ownership. The complete unit and renewal procedure are in the [watchdog guide](https://github.com/mxh110708/may-vps-deploy/blob/main/docs/MONITOR-WATCHDOG.zh-CN.md).

## Upgrade and recover

Record exact versions, asset digests, installation identity, current runtime state, read-grant expiry and theme lock before changing anything. Upgrade Server before paired Agents; Web is normally embedded, while the theme is separately pinned.

Take a consistent component snapshot, including configuration, databases/WAL and referenced theme material. Backing up only an actively written SQLite main file is insufficient. Verify archives before extraction. On cancellation or uncertain acknowledgement, reconcile the same transaction instead of replaying it.

Restore only the chosen component and retain current data before overwriting it. Check service/version, UUID, HTTPS, credentials, history and fresh reports before committing recovery. Do not restart or restore tunnels, proxies and SSH as a side effect of a controller-only restore.

## Alerts and diagnosis

Email is opt-in and needs a private SMTP configuration plus an enabled channel and policy. Local TLS fixture acceptance is not proof of real inbox delivery. See [email operations](EMAIL.zh-CN.md).

For missing metrics, inspect scope, Agent credentials, report age and storage. For 401 responses, inspect read-token format, digest, expiry and the actual process environment. For blank pages, inspect asset content/MIME and theme lock; HTML returned with 200 is not a successful JS request. For growing files, compare DB and WAL separately across equivalent phases, retention and checkpoint timing.

Do not upload databases, full backups, tokens or SMTP files in issue reports. For a forgotten password, inspect private records and the authorized recovery process. The legacy `chpasswd` command accepts and prints a password, so its output and argument exposure require an isolated operator procedure; deleting the database is not password recovery.
