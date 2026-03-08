# Release & Homebrew Distribution Plan

## Context

`ifc-to-db` uses goreleaser v2 for release automation and distributes via a custom Homebrew tap.

Three prerequisites before first release:
1. Fix go.mod module path (local `ifc-cli` → `github.com/johannesmichael/ifc-cli`)
2. Add version injection via ldflags
3. Set up tap repo + GitHub secret

**Note on CGO**: go-duckdb requires CGO, but is not yet wired up (`CGO_ENABLED=0` currently works). When DuckDB is integrated, release builds will need native runners per platform. See [CGO Transition](#cgo-transition) below.

---

## One-Time Setup

### 1. Create the Homebrew Tap Repo

Create `johannesmichael/homebrew-ifc-cli` on GitHub (public, empty).

goreleaser will auto-populate `Formula/ifc-to-db.rb` on first release.

### 2. Create a GitHub PAT

Create a Personal Access Token with `repo` scope. Save it as `HOMEBREW_TAP_GITHUB_TOKEN` in the `ifc-cli` repo's Actions secrets (Settings → Secrets → Actions).

---

## Making a Release

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions triggers goreleaser, which:
- Builds for linux/darwin × amd64/arm64 + windows/amd64
- Creates GitHub release with `.tar.gz`/`.zip` archives and SHA-256 checksums
- Commits updated `Formula/ifc-to-db.rb` to `homebrew-ifc-cli`

---

## Install via Homebrew (after first release)

```bash
brew tap johannesmichael/ifc-cli
brew install ifc-to-db
```

---

## Local Development Builds

```bash
make build       # builds with version injected from git describe
make snapshot    # goreleaser snapshot build (no publish) → dist/
make release-dry # goreleaser dry-run release (no publish)
```

---

## Files Changed to Enable This

| File | Change |
|------|--------|
| `go.mod` | `module ifc-cli` → `module github.com/johannesmichael/ifc-cli` |
| All `*.go` internal imports | `"ifc-cli/` → `"github.com/johannesmichael/ifc-cli/` |
| `Makefile` | Add `VERSION`/`BUILD_DATE`/`LDFLAGS` vars; update `build`; add `snapshot`, `release-dry`; remove old `release` target |
| `.github/workflows/build.yml` | Replace build matrix + release jobs with goreleaser job on tags; add lightweight build check for PRs/pushes |
| `.goreleaser.yml` | New — goreleaser v2 config |

---

## goreleaser Config Reference

```yaml
version: 2

project_name: ifc-to-db

before:
  hooks:
    - go mod tidy

builds:
  - id: ifc-to-db
    main: ./cmd/ifc-to-db
    binary: ifc-to-db
    env:
      - CGO_ENABLED=0
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w -trimpath
      - -X github.com/johannesmichael/ifc-cli/internal/cli.Version={{.Version}}
      - -X github.com/johannesmichael/ifc-cli/internal/cli.BuildDate={{.Date}}

archives:
  - id: default
    name_template: "ifc-to-db-{{ .Os }}-{{ .Arch }}"
    format_overrides:
      - goos: windows
        formats: [zip]

checksum:
  name_template: checksums-sha256.txt
  algorithm: sha256

release:
  github:
    owner: johannesmichael
    name: ifc-cli
  generate_release_notes: true

brews:
  - name: ifc-to-db
    repository:
      owner: johannesmichael
      name: homebrew-ifc-cli
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    commit_author:
      name: johannesmichael
      email: noreply@github.com
    homepage: "https://github.com/johannesmichael/ifc-cli"
    description: "Parse IFC files and write contents to DuckDB for SQL analysis"
    license: "MIT"
    install: |
      bin.install "ifc-to-db"
    test: |
      system "#{bin}/ifc-to-db", "version"
```

---

## Verification Checklist

```bash
# After module path rename
go build ./...
go test ./...

# Check version injection
./bin/ifc-to-db version
# Expected: ifc-to-db v0.1.0-<sha> (or tag if on a tag)

# Validate goreleaser config
goreleaser check
goreleaser build --snapshot --clean
ls dist/   # should contain 4 platform binaries

# After first tag push — check tap repo for Formula/ifc-to-db.rb
# Then test install:
brew tap johannesmichael/ifc-cli
brew install ifc-to-db
ifc-to-db version
```

---

## CGO Transition

When go-duckdb is integrated (`CGO_ENABLED=1`), cross-compilation no longer works from a single Linux runner. Replace the goreleaser job with a native matrix:

| Platform | GitHub Runner |
|----------|--------------|
| darwin/arm64 | `macos-latest` |
| darwin/amd64 | `macos-13` |
| linux/amd64 | `ubuntu-latest` |
| linux/arm64 | `ubuntu-24.04-arm` |
| windows/amd64 | `windows-latest` |

Use goreleaser `--split` to build on each native runner, then merge artifacts in a final step. See goreleaser docs on [split builds](https://goreleaser.com/customization/split/).
