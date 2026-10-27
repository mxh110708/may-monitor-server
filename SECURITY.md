# May Monitor server security profile

The 2026-10-24 review covers the default monitoring profile at the locally
reviewed source snapshot. It is not approval to deploy all upstream features.
The workspace report records exact evidence, upstream commits and limitations.

- Sessions and agent tokens are revalidated on WebSocket messages. Revoked or
  expired credentials cannot continue sending reports or receiving control data.
- Agent inventory accepts machine fields only. Visibility, tokens, notes,
  names, billing and group assignments belong to the administrator.
- JSON and decompressed WebSocket/body messages are limited to 4 MiB; RPC
  batches to 100 calls. File streaming uses separate authenticated limits.
- Passwords use randomly salted bcrypt over SHA-256 prehashed password input,
  work factor 12. Retired fixed-salt hashes are rejected without rewriting the
  account; the normal administrator password-reset procedure remains available.
- Login attempts, concurrent password verification and OAuth state storage are
  bounded. Forwarded IP headers are untrusted unless the operator configures
  `MAY_MONITOR_TRUSTED_PROXIES` with actual proxy addresses/CIDRs.
- First-run writes require `X-May-Setup-Token`, obtained from the private
  `/install#setup=...` link printed locally at startup. A token may instead be
  supplied through `MAY_MONITOR_SETUP_TOKEN` (32-256 characters). Keep that
  console output private. The token is not returned by an API and becomes
  unusable after the installation controller deactivates. A fresh installation
  starts in private-site mode with marketplace SSRF protection enabled.
- The normal data root is private (Unix mode 0700), and a symlink root is
  rejected. External database paths, service accounts, file ACLs, reverse proxy
  admission limits and backups still require deployment review.

## Operator-controlled capabilities

The following environment capabilities default to **disabled** and cannot be
enabled using web settings or an API key:

| Variable | Capability |
| --- | --- |
| `MAY_MONITOR_ALLOW_REMOTE_CONTROL` | Agent commands, terminal, file access, startup configuration and version switching |
| `MAY_MONITOR_ALLOW_DATABASE_CONSOLE` | Arbitrary SQL queries and statements |
| `MAY_MONITOR_ALLOW_EXTENSIONS` | Plugins, custom themes/raw page HTML and JavaScript notification scripts |
| `MAY_MONITOR_ALLOW_ARCHIVE_IMPORT` | Archive import; extension permission is also required |
| `MAY_MONITOR_ALLOW_DEBUG` | Profiling/debug HTTP routes |

Only the exact value `true` opts in. Enabling a capability expands the trusted
administrator and code execution boundary; it does not inherit approval from
the monitoring-profile review. API keys remain full administrator credentials,
not scoped read-only integration tokens. Do not use them for a public client.

The post-rc.1 source adds an independently authenticated, disabled-by-default
read-only metrics endpoint. It requires an operator-configured token digest,
explicit client UUIDs and an expiration, and never promotes its bearer namespace
to an administrator principal. See [the read-only integration contract](docs/READ-ONLY-INTEGRATION.zh-CN.md).

The paired Deploy lifecycle uses dedicated service users, loopback-only server
listeners and component-scoped consistency snapshots. Its agent credential
confirmation endpoint returns only the authenticated agent UUID and server
version, without registration or synthetic reporting. Operator-provided setup
tokens are not printed to service logs. See [the lifecycle contract](docs/DEPLOY-LIFECYCLE.zh-CN.md)
for its disposable-system verification and production migration limits.

RC3 adds an opaque operator-managed installation UUID to already
authenticated snapshots and agent confirmation. It is an origin comparison
label, not an authorization secret or hardware attestation. Missing identifiers
cannot prove an origin; the paired client still validates TLS and each node's
credential scope. Copying the entire identity and dataset is an explicit
operator change outside an uncopyable-identity guarantee.

The retained JavaScript runtime is not an operating-system security boundary.
Untrusted plugins and notification scripts must not be loaded. Optional real
SSO providers, remote control and data migration still need separate acceptance.

## Builds and upstream changes

Monitoring email delivery is an explicit, separate channel. Its private SMTP
file is not an administrator setting or read-only API response. Only verified
TLS on ports 465/587 with authentication is supported; public DNS results are
pinned for dialing and private/loopback destinations are rejected. Delivery
has a ten-second bound and reuses the bounded event/retry queue. Local HTTP
receivers remain loopback-only. See [email operations](docs/EMAIL.zh-CN.md).

Use Go 1.27.3 and the reviewed frontend revision/lockfile. The manual security
workflow accepts a full commit ID from the owned May frontend repository and
uses pinned official Actions. It does not publish or deploy. Historical upstream
workflows live under `.github/upstream-workflows/` with disabled extensions.
`install-komari.sh.upstream-disabled` is reference material, not an installer.
Do not restore upstream auto-follow or release automation without reviewing it.

The known OpenPGP warning in `golang.org/x/crypto` has no fixed upstream version;
the server does not import those packages. Keep the warning visible and verify
the import graph again after every dependency or source change.

## First May prerelease

`v0.1.0-rc.1` packages this reviewed monitoring profile. See the [shared security review](https://github.com/mxh110708/may-monitor-server/blob/v0.1.0-rc.1/docs/SECURITY-REVIEW-2026-10-24.zh-CN.md). Release manifests pin all three source commits. No automatic installation or production migration is authorized.
