# Build the May Monitor prerelease

Use the tag's `frontend.lock.json`, Go 1.27.3 and Node 22.23.4. Preserve upstream
licenses and the default security policy. Old upstream installers/workflows
with disabled extensions are reference material and must not be executed.

1. Check out the exact web repository and commit recorded in `frontend.lock.json`.
2. In that checkout, run `npm ci --ignore-scripts`, the security/onboarding
   tests and lint. Set `SOURCE_DATE_EPOCH` to the web commit timestamp and run
   `npm run build`.
3. From the server checkout, run
   `go run ./tools/packfrontend <web-checkout>/dist web/public/defaultTheme/dist.tar.zst`
   and copy its `komari-theme.json` to `web/public/defaultTheme/komari-theme.json`.
4. Run module integrity, full Go tests and vulnerability checks. The documented
   unused OpenPGP module warning is retained; review import reachability.
5. Build with `-trimpath` and set `utils.CurrentVersion` and `utils.VersionHash`
   to the release version and exact server commit. The module prefix remains
   `github.com/komari-monitor/komari` for source compatibility and attribution.

Linux release binaries use Zig 0.14.1 as the C compiler with the appropriate
`x86_64-linux-musl` or `aarch64-linux-musl` target, CGO enabled, the `netgo,osusergo`
tags and external static linking. Verify the official compiler archive checksum
before use. This avoids a dependency on the build host's glibc version.
ARM cross-compilation alone does not verify native ARM behavior.

Release manifests record source commits, tool versions and artifact digests.
Publishing a prerelease does not install it or authorize production migration.
