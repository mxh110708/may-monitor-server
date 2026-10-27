# May Monitor Server 0.1.0

First stable release of the reviewed May monitoring profile. Promote the RC9
implementation without functional changes: scoped and expiring read access,
installation identity checks, bounded telemetry and probe history, confirmed
alerts, protected SMTP delivery, and an independent Linux watchdog with
exclusive state ownership. Remote control, arbitrary SQL, extensions and debug
capabilities remain disabled by default.

Pair with May Agent 0.1.0, May Web 0.1.0 and reviewed May Status 2.2.2-may.17.
Current Debian 12/13 amd64 operations are the acceptance target; other artifacts
are cross-compiled only. Retain the exact M5 frozen-test report, its single
historical anomaly and the documented snapshot capture-order repair. Do not
claim a new 24-hour run of this promoted binary.

Email is deliberately unconfigured at the operator's request. TLS SMTP fixture
acceptance is recorded separately from real inbox delivery. Configure sender,
recipients and a private SMTP authorization before enabling real email; no
external test message has been sent. See docs/EMAIL.zh-CN.md.

Keep current May backups, schema maintenance and component-scoped recovery.
Completed Komari migration import bridges remain retired; upstream attribution
and monitoring protocol identities remain intact. Known dependency notices and
review boundaries are retained in SECURITY.md. Publication does not install
Windows desktop updates or alter unrelated services.
