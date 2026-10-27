# May Monitor Server 0.1.0-rc.7

This prerelease includes the reviewed M4–M9 monitoring work: scoped expiring read grants, installation identity checks, report freshness, notifications and the operator-pinned static May Status theme. The embedded Web UI is locked to an exact May Web 0.1.0-rc.3 commit. Remote execution, arbitrary SQL, general extensions and archive imports stay disabled in the default profile.

After the completed fresh May cutover, retire the fixed-salt Komari password fallback, old configuration/timestamp/client/history import engines and the old notification-upgrade RPC. Unsupported pre-May database layouts now fail before AutoMigrate instead of being imported or silently ignored. Current May schema upgrades, administrator-only metric-store recovery, password reset, version-change backups and component-scoped rollback remain supported.

The snapshot arrival-boundary regression reproduces the former negative-age failure against the old ordering and verifies capture after report assembly. The original frozen 24-hour observation still contains its one historical NeedsReview point; it is not rewritten as a zero-anomaly run. The new release does not claim another 24-hour test or measured resource improvement over RC6.

Publish with paired Agent 0.1.0-rc.4, Web 0.1.0-rc.3 and May Status 2.2.2-may.15. Publishing does not upgrade production. Preserve the source licenses and the documented non-reachable OpenPGP dependency advisory; a module advisory is not a proven reachable issue in this monitoring profile.
