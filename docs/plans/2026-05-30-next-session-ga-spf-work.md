# Dmark GA SPF Flattening Next Session Implementation Plan

> **For Hermes:** Use subagent-driven-development skill to implement this plan task-by-task. Also load test-driven-development, github-pr-workflow, and memento-vault before starting. Work backend-first in `dmark-app`, then hand off only the needed UI/proxy work to `dmark-web`.

**Goal:** In the next fresh session, complete a meaningful first implementation slice of Dmark's GA-blocking SPF flattening service: core recursive SPF resolver, deterministic SPF generator, and nameserver provider autopicker primitives, all under tests.

**Architecture:** Start with pure Go packages that have no database, Bunny.net, or HTTP coupling. Implement `internal/spfflatten` for SPF resolution/generation and `internal/dnsprovider` for authoritative-NS provider detection/instruction metadata. Keep side effects behind interfaces so later slices can add persistence, Bunny publication, workers, and dashboard APIs safely.

**Tech Stack:** Go 1.25.10 via Docker on Havana, standard library DNS/test fakes, existing `dmark-app` package layout, `go test ./...`, `go vet ./...`, and eventual sqlc/Postgres integration.

---

## Fresh Session Boot Checklist

1. Recall context from memento before touching files:
   - Query: `dmark GA SPF flattening Bunny nameserver provider autopicker next session`
   - Query: `dmark-app SPF_FLATTENER_BUNNY_PLAN GA_READINESS`
2. Check repo state:
   ```bash
   cd /root/workspace/github/AndriGitDev/dmark-app
   git status --short --branch
   git pull --ff-only
   ```
3. Read the canonical docs:
   - `docs/GA_READINESS.md`
   - `docs/SPF_FLATTENER_BUNNY_PLAN.md`
4. Remember environment constraint: native `go` may be missing; use Docker:
   ```bash
   docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go test ./...
   ```
5. Do not require Bunny credentials for this slice. The next session should ship tested offline primitives first.

---

## Slice Boundary For The Next Session

### In scope

- Pure recursive SPF resolver with fake DNS TXT resolver.
- Deterministic SPF string generator.
- Oversized SPF blocking guard.
- Nameserver provider detector with Icelandic-first registry.
- Tests for all above.
- Commit and push to `main` if tests pass.
- Capture the push in memento.

### Out of scope for this next slice

- Postgres migrations and store implementation.
- Bunny.net live API client.
- River worker scheduling.
- Dashboard HTTP endpoints.
- `dmark-web` UI implementation.

Reason: the pure primitives de-risk the hardest logic and keep the first work chunk shippable without credentials.

---

## Task 1: Create SPF flattener package skeleton

**Objective:** Add the package where resolver and generator code will live.

**Files:**
- Create: `internal/spfflatten/spfflatten.go`
- Create: `internal/spfflatten/spfflatten_test.go`

**Step 1: Create the initial test file**

Add a trivial compile test so the package exists:

```go
package spfflatten

import "testing"

func TestPackageCompiles(t *testing.T) {
    t.Parallel()
}
```

**Step 2: Create the package file**

```go
package spfflatten
```

**Step 3: Verify**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go test ./internal/spfflatten
```

Expected: PASS.

**Step 4: Commit**

```bash
git add internal/spfflatten
git commit -m "feat: add SPF flattener package skeleton"
```

---

## Task 2: Add core SPF resolver types and fake DNS interface

**Objective:** Define input/output types before implementing resolver behaviour.

**Files:**
- Modify: `internal/spfflatten/spfflatten.go`
- Modify: `internal/spfflatten/spfflatten_test.go`

**Required types:**

```go
type TXTResolver interface {
    LookupTXT(ctx context.Context, domain string) ([]string, error)
}

type Source struct {
    Kind  SourceKind
    Value string
    Label string
}

type SourceKind string

const (
    SourceInclude SourceKind = "include"
    SourceIP4     SourceKind = "ip4"
    SourceIP6     SourceKind = "ip6"
)

type Mechanism struct {
    Kind   SourceKind
    Value  string
    Source string
}

type Evidence struct {
    SourceValue    string
    ResolvedDomain string
    Mechanism      string
    Kind           string
    Message        string
}

type Result struct {
    Mechanisms []Mechanism
    Evidence   []Evidence
    Errors     []ResolutionError
}

type ResolutionError struct {
    Domain  string
    Message string
}

func (r Result) Blocked() bool { return len(r.Errors) > 0 }
```

**Step 1: Write tests for literal source passthrough**

Test that an `ip4` and `ip6` source produce mechanisms without DNS lookups.

**Step 2: Implement minimal resolver**

Add:

```go
type Resolver struct {
    DNS      TXTResolver
    MaxDepth int
}

func (r Resolver) Resolve(ctx context.Context, sources []Source) Result
```

For this task, only handle literal `ip4` and `ip6`.

**Step 3: Verify**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go test ./internal/spfflatten
```

Expected: PASS.

**Step 4: Commit**

```bash
git add internal/spfflatten
git commit -m "feat: add SPF resolver core types"
```

---

## Task 3: Resolve include domains and extract IP mechanisms

**Objective:** Resolve one SPF include domain and extract `ip4:` / `ip6:` mechanisms.

**Files:**
- Modify: `internal/spfflatten/spfflatten.go`
- Modify: `internal/spfflatten/spfflatten_test.go`

**Step 1: Write failing tests**

Cases:

- `_spf.example.test` returns `v=spf1 ip4:192.0.2.10 ip6:2001:db8::/32 ~all`.
- Resolver returns two mechanisms.
- Evidence identifies `_spf.example.test` as the resolved domain/source.

Use a fake resolver map:

```go
type fakeTXT map[string][]string

func (f fakeTXT) LookupTXT(ctx context.Context, domain string) ([]string, error) {
    if records, ok := f[domain]; ok {
        return records, nil
    }
    return nil, nil
}
```

**Step 2: Implement SPF record selection**

Rules:

- Join TXT fragments if necessary.
- Select records beginning with `v=spf1`.
- Zero SPF records => blocking error.
- Multiple SPF records => blocking error.

**Step 3: Implement mechanism extraction**

Parse whitespace-delimited tokens.
Extract only:

- `ip4:<value>`
- `ip6:<value>`

Ignore terminal mechanisms for now except preserving them later in generator config.

**Step 4: Verify**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go test ./internal/spfflatten
```

Expected: PASS.

**Step 5: Commit**

```bash
git add internal/spfflatten
git commit -m "feat: resolve SPF include mechanisms"
```

---

## Task 4: Add nested include recursion, loop detection, and depth guard

**Objective:** Make resolver safe against recursive include chains.

**Files:**
- Modify: `internal/spfflatten/spfflatten.go`
- Modify: `internal/spfflatten/spfflatten_test.go`

**Step 1: Write failing tests**

Cases:

1. Nested include:
   - `a.test` => `v=spf1 include:b.test ~all`
   - `b.test` => `v=spf1 ip4:192.0.2.20 ~all`
   - Expected: one `ip4` mechanism.
2. Include loop:
   - `a.test` includes `b.test`
   - `b.test` includes `a.test`
   - Expected: `Blocked() == true` and error mentions loop.
3. Max depth:
   - chain longer than default 10
   - Expected: blocked with depth error.

**Step 2: Implement recursion**

Implementation notes:

- Default `MaxDepth` to 10 when unset.
- Track visited domains per resolution.
- Normalize domains with `strings.ToLower(strings.TrimSuffix(strings.TrimSpace(domain), "."))`.
- Preserve enough evidence to explain include path.

**Step 3: Verify**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go test ./internal/spfflatten
```

Expected: PASS.

**Step 4: Commit**

```bash
git add internal/spfflatten
git commit -m "feat: add recursive SPF include resolution"
```

---

## Task 5: Add deterministic SPF generator and oversized blocking

**Objective:** Convert resolved mechanisms into a stable SPF TXT string.

**Files:**
- Create: `internal/spfflatten/generator.go`
- Create or modify: `internal/spfflatten/generator_test.go`

**Step 1: Write failing tests**

Cases:

1. Deduplicates duplicate mechanisms.
2. Sorts deterministically: all `ip4` lexicographically, then all `ip6` lexicographically.
3. Emits `v=spf1 ... ~all` by default.
4. Supports explicit terminal mechanism `-all` only when configured.
5. Blocks when output exceeds configured max length.

**Proposed API:**

```go
type GenerateOptions struct {
    Terminal string
    MaxLength int
}

type Generated struct {
    SPF     string
    Blocked bool
    Error   string
}

func Generate(mechanisms []Mechanism, opts GenerateOptions) Generated
```

**Step 2: Implement minimal generator**

- Default terminal: `~all`.
- Default max length: 450 bytes unless plan/codebase suggests better.
- Reject unsupported terminal values except `~all`, `-all`, `?all`.

**Step 3: Verify**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go test ./internal/spfflatten
```

Expected: PASS.

**Step 4: Commit**

```bash
git add internal/spfflatten
git commit -m "feat: generate deterministic flattened SPF records"
```

---

## Task 6: Create DNS provider autopicker package skeleton

**Objective:** Add server-side provider detection primitives independent of SPF publishing.

**Files:**
- Create: `internal/dnsprovider/provider.go`
- Create: `internal/dnsprovider/provider_test.go`

**Required model:**

```go
type Status string

const (
    StatusDetected  Status = "detected"
    StatusAmbiguous Status = "ambiguous"
    StatusUnknown   Status = "unknown"
)

type Provider struct {
    ID           string
    Name         string
    NSPatterns   []string
    DashboardURL string
}

type Detection struct {
    Status              Status
    ProviderID          string
    ProviderName        string
    Confidence          string
    EvidenceNameservers []string
    DNSDashboardURL     string
}
```

**Step 1: Write failing tests**

Cases:

- ISNIC nameservers detect `isnic`.
- Cloudflare nameservers detect `cloudflare`.
- Unknown nameservers return `unknown`.
- Mixed nameservers matching two providers return `ambiguous`.

**Step 2: Implement registry and detector**

Initial registry must include:

- `isnic`
- `1984`
- `opin-kerfi`
- `origo`
- `advania`
- `cloudflare`
- `route53`
- `bunny`

Use conservative pattern matching. If unsure of exact Icelandic NS patterns, include placeholder TODO comments but keep tests around fixture patterns discovered from current provider docs later. Do not claim high confidence without exact pattern evidence.

**Step 3: Verify**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go test ./internal/dnsprovider
```

Expected: PASS.

**Step 4: Commit**

```bash
git add internal/dnsprovider
git commit -m "feat: add DNS provider autopicker primitives"
```

---

## Task 7: Add provider instruction model for DNS record types

**Objective:** Prepare backend-ready copy/instruction structures for dashboard use.

**Files:**
- Modify: `internal/dnsprovider/provider.go`
- Modify: `internal/dnsprovider/provider_test.go`

**Record types:**

```go
type RecordKind string

const (
    RecordDMARCTXT    RecordKind = "dmarc_txt"
    RecordSPFTXT      RecordKind = "spf_txt"
    RecordSPFRouteCNAME RecordKind = "spf_route_cname"
)
```

**Step 1: Write failing tests**

Cases:

- Detected Cloudflare returns generic dashboard URL and 3-5 instruction steps for SPF route CNAME.
- Unknown provider returns generic DNS instructions.
- Instructions include record name, type, value, and TTL where relevant.

**Step 2: Implement instruction rendering**

Suggested API:

```go
type RecordInstructionInput struct {
    Kind  RecordKind
    Name  string
    Value string
    TTL   int
}

type Instructions struct {
    ProviderID      string
    ProviderName    string
    DashboardURL    string
    Steps           []string
}

func BuildInstructions(d Detection, input RecordInstructionInput) Instructions
```

**Step 3: Verify**

Run:

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go test ./internal/dnsprovider
```

Expected: PASS.

**Step 4: Commit**

```bash
git add internal/dnsprovider
git commit -m "feat: add DNS provider record instructions"
```

---

## Task 8: Run repo-level verification

**Objective:** Prove the new primitives do not break existing backend code.

**Files:**
- No new files unless fixes are needed.

**Step 1: Run focused tests**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go test ./internal/spfflatten ./internal/dnsprovider
```

Expected: PASS.

**Step 2: Run full backend tests**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go test ./...
```

Expected: PASS.

**Step 3: Run vet**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go vet ./...
```

Expected: PASS.

**Step 4: Run build**

```bash
docker run --rm -v "$PWD":/src -w /src golang:1.25.10 go build -o /tmp/dmark-build-check ./cmd/meil
```

Expected: PASS.

**Step 5: Check formatting and diff hygiene**

```bash
git diff --check
git status --short
```

Expected: no whitespace errors, only intended files changed if not already committed.

---

## Task 9: Push and capture

**Objective:** Leave the project in a clean, recoverable state for the following session.

**Step 1: Squash is not required**

Frequent commits are acceptable because this is direct implementation work on `main` unless the next operator chooses to branch.

**Step 2: Push**

```bash
git push
```

**Step 3: Capture to memento**

Record:

- repo: `dmark-app`
- branch: `main`
- commits pushed
- files changed
- test commands and whether they passed
- any blockers such as missing exact Icelandic provider NS patterns or missing Bunny credentials

---

## If There Is Extra Time

After the pure primitives are green, continue in this order:

1. Add Postgres migration `migrations/postgres/00006_spf_routes.sql`.
2. Add store methods for SPF route config/state.
3. Add sync service that combines `spfflatten.Resolver`, `spfflatten.Generate`, and persistence behind a Bunny publisher interface.
4. Add fake Bunny publisher tests.
5. Only then start dashboard API endpoints.

Do not start Bunny live API calls until `BUNNY_DNS_API_KEY`, `BUNNY_DNS_ZONE_ID`, and `DMARK_SPF_ROUTE_ZONE` are present and scoped for a test zone.

---

## Acceptance Criteria For The Next Session

A good next session should end with:

- `internal/spfflatten` implemented and tested for literals, includes, nested includes, loops, max depth, deterministic generation, and oversized blocking.
- `internal/dnsprovider` implemented and tested for provider detection and instruction rendering.
- Full backend `go test ./...`, `go vet ./...`, and `go build ./cmd/meil` passing via Docker.
- Changes committed and pushed to `main`.
- Memento capture written.

If time is short, prioritize `internal/spfflatten` over provider autopicker. SPF flattening is the harder GA blocker and unlocks persistence/Bunny work next.
