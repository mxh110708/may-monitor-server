# Changelog

## 0.1.0-rc.1 — 2026-10-24

Harden first-run enrollment, sessions and WebSocket revocation, agent field ownership, password storage, OAuth state and request limits. Disable remote control, arbitrary SQL, extensions and archive imports by default. Pin the compatible frontend; retain the documented unused OpenPGP advisory.

Initial prerelease based on Komari. Security defaults remain enabled. Existing Komari, Deploy and Route installations are not modified by publishing this release. See SECURITY.md and the shared review for test coverage and limits.

Release validation also fixes prerelease plugin-version parsing and rejects unknown version claims instead of bypassing compatibility checks.
