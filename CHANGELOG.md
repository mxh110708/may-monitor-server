# Changelog

## Unreleased

Add an optional opaque installation identity to authenticated snapshots and agent credential confirmation. The paired Deploy workflow compares SSH ownership with the HTTPS origin and verifies advancing telemetry before marking a node ready for trial. Keep schema 1 readers compatible; unset identities cannot prove an origin. No hardware identity, administrator privilege, automatic migration or upstream update is added.

Add a default-off, scoped and expiring read-only metrics integration for May Deploy. Keep credential namespaces separate, return only registered authorized nodes and scalar metrics, and retain private-site, administrator and agent boundaries. Add router-level, malformed-scope and revocation regressions. No automatic installation or migration.

Add a non-mutating agent credential confirmation endpoint for the paired Deploy lifecycle. Keep operator-provided first-run setup tokens out of service logs. Use transport-specific online state: WebSocket reports no longer renew POST fallback leases, and reconnecting WebSockets retire their old fallback timer. Validate installation, RC1 upgrades, backups, rollback, restart and real agent metrics on disposable Debian 12/13 systems; production migration remains an explicit per-node operation.

## 0.1.0-rc.1 — 2026-10-24

Harden first-run enrollment, sessions and WebSocket revocation, agent field ownership, password storage, OAuth state and request limits. Disable remote control, arbitrary SQL, extensions and archive imports by default. Pin the compatible frontend; retain the documented unused OpenPGP advisory.

Initial prerelease based on Komari. Security defaults remain enabled. Existing Komari, Deploy and Route installations are not modified by publishing this release. See SECURITY.md and the shared review for test coverage and limits.

Release validation also fixes prerelease plugin-version parsing and rejects unknown version claims instead of bypassing compatibility checks.
