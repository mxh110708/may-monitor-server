# may-monitor-server v0.1.0-rc.9

Add optional SMTP delivery to monitoring alerts and the independent watchdog.
Require authenticated TLS 465 or mandatory STARTTLS 587, verified certificates,
public pinned destination addresses and a private configuration file. Preserve
event IDs, bounded retries, silence and recovery behavior; do not expose SMTP
credentials through settings or read-only summaries. Local HTTP delivery stays
restricted to loopback. Email is unconfigured and does not send by default.

Add Linux process locking for watchdog state so concurrent processes cannot
corrupt the same event state. Pair with May Web 0.1.0-rc.5. The email setup guide
distinguishes local TLS fixture acceptance from real inbox delivery; the latter
has been explicitly deferred by the operator.

Publishing does not install desktop updates or independently authorize changes
to other services. Production upgrade evidence belongs in the Deploy closeout
record. Frozen M5 evidence and its single historical anomaly remain unchanged.
