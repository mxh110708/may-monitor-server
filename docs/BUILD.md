# Build May Monitor Server

Baseline: the 0.1.0 release family. Use the selected tag's `frontend.lock.json`, Go **1.27.3**, Node **22.23.4** and locked dependencies. Preserve upstream licensing and the default capability policy. This is a development workflow, not an installation or deployment script.

## 1. Build the pinned frontend

Read `frontend.lock.json` and check out exactly that commit of `mxh110708/may-monitor-web`, including its lockfile. Do not substitute an upstream branch, a local uncommitted dist or the public May Status theme.

In that checkout:

```bash
npm ci --ignore-scripts
npm run lint
npm run test:security
npm run test:onboarding
npm run test:monitoring
npm run build
npm run test:service-worker
```

For reproducible packaging, set `SOURCE_DATE_EPOCH` to the pinned Web commit timestamp before building. Service-worker validation runs after build and checks that private API responses are not cached. Existing lint warnings must be assessed separately from errors; a passing command is not browser acceptance.

## 2. Embed Web into Server

From the Server checkout, replace `<web-checkout>` with the verified checkout path:

```bash
go run ./tools/packfrontend '<web-checkout>/dist' web/public/defaultTheme/dist.tar.zst
```

Copy the same Web checkout's `komari-theme.json` into `web/public/defaultTheme/komari-theme.json`. Both embedded files must agree with the lock. A missing asset must not silently be substituted by an HTML SPA fallback.

## 3. Verify Server

```bash
go mod verify
go test -count=1 -timeout=5m ./...
```

Run relevant race tests and the pinned vulnerability workflow for source or dependency changes. Keep the documented OpenPGP advisory visible and verify actual import reachability; do not suppress it merely because the package is currently unused. See [SECURITY.md](../SECURITY.md) and `.github/workflows/security.yml` for the reviewed checks.

## 4. Build and record provenance

Use `-trimpath` and set `github.com/komari-monitor/komari/utils.CurrentVersion` and `VersionHash` to the intended version and exact Server commit. Retaining the module prefix does not retain a legacy product name or enable old capabilities.

The Linux release profile uses Zig **0.14.1**, CGO, `netgo,osusergo`, external static linking and the appropriate `x86_64-linux-musl` or `aarch64-linux-musl` target. Verify the compiler archive before use. A plain local `go build` is useful for development but does not reproduce the release ABI profile. Cross-compilation does not prove native ARM runtime behavior.

Record Server/Web commits, tool versions, asset names, sizes and SHA-256 in the release manifest. Pair the versioned release note with the artifacts. Do not overwrite an already published archive under the same version.

## 5. Validate at the right level

Unit and race tests validate code paths; temporary Debian lifecycle tests validate accounts, service state and recovery; real Agent reports and HTTPS verify a specific installation; a soak requires real elapsed time and retained evidence. Historical RC4/RC2 M5 results do not become a fresh 0.1.0 soak by rebuilding the binary.

Pure documentation changes need local links, source-contract/example checks and secret review, not a new binary version, upstream release, browser cache reset or production restart.
