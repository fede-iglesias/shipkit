# shipkit Implementation Master Roadmap

Status: ready for autonomous Sonnet execution via architect-coordinator skill.
Target: shipkit v0.1.0 (multi-module mono-repo) + kt v0.2.0 (consumer migration).
Date: 2026-06-04

## Provenance

This roadmap was produced by 4 Opus 4.7 architects: 3 ran in parallel (A lifecycle verbs, B distribution pipeline, C migration TDD strategy) and a 4th consolidated their outputs after applying 9 pre-decided open-question resolutions from the orchestrator.

Full consolidated plan lives in engram (NOT inline here):

| Artifact | Engram topic_key | Obs ID | Project |
|----------|------------------|--------|---------|
| Consolidated plan sections 1-8 | `shipkit/plan/consolidated` | 5152 | kt |
| Consolidated plan sections 9-12 | `shipkit/plan/consolidated-tail` | 5153 | kt |
| Opus A lifecycle verbs design | `shipkit/plan/lifecycle-verbs` | 5149 | kt |
| Opus B distribution pipeline design | `shipkit/plan/distribution` | 5148 | kt |
| Opus C migration TDD strategy | `shipkit/plan/migration-tdd` | 5150 | kt |
| 3-Opus angle summary | `shipkit/plan/three-opus-summary` | 5151 | kt |
| shipkit local discovery | `kt/shipkit-discovery` | 5147 | kt |
| **In-code docs preference (user override)** | `shipkit/preference/in-code-docs` | 5154 | kt |
| **In-code docs standards (mandatory spec)** | `shipkit/standards/in-code-docs` | 5155 | kt |

Note: engram project is `kt` because shipkit is not yet registered. All shipkit topic_keys are under the kt project.

## Goal

Extract 4 reusable packages from kt v0.1.3 into a new public multi-module mono-repo `fede-iglesias/shipkit` and design 4 new lifecycle verbs from scratch with strict TDD. Final result: any personal Go CLI can register the full self-contained lifecycle (install, update, uninstall, doctor, clean) with one call to `shipkit.RegisterLifecycle(root, cfg)`, distributed via `fede-iglesias/tools` public repo using cosign keyless signs. ZERO brew, deb, snap, krew, docker.

## Scope (locked, do not expand)

Extract (4 packages):
1. `shipkit/frontmatter` (entire kt pkg/frontmatter)
2. `shipkit/lifecycle/migrations` (entire kt pkg/migrations)
3. `shipkit/lifecycle/update` (entire kt pkg/upgrade, renamed)
4. `shipkit/store` (PARTIAL: lock + path + checksum + atomic primitives only; kt-internal split first)

Create from scratch with TDD:
5. `shipkit/lifecycle/install`
6. `shipkit/lifecycle/uninstall`
7. `shipkit/lifecycle/doctor`
8. `shipkit/lifecycle/clean`

Public API:
9. `shipkit.Config` struct + `RegisterLifecycle(root, cfg, opts...)` hybrid cobra integration + per-verb getters + 11 Ports + production Adapters.

Validation:
10. `shipkit/example/shipkit-example/` minimal CLI as second consumer for integration tests (NO release to tools repo in v0.1.0).

Consumer migration:
11. `kt v0.2.0` swaps local pkg imports to shipkit, wires `shipkit.RegisterLifecycle`, keeps kt-specific code (cards, hooks, audit).

OUT OF SCOPE for v0.1.0 (do NOT add):
- UI components, theme, screens, TUIs
- shipkit.Build(App) facade
- pkg/hooks extraction (kt-specific)
- pkg/importer extraction (does not exist in kt)
- Releasing shipkit-example to tools repo
- claudeq as consumer (deferred per user decision)

## Dependency graph

```
B0 (bootstrap)
  |
  v
B1 (parallel: frontmatter, migrations)
  |
  v
B1.5 (kt barrier: import swap + smoke)
  |
  v
B2 (waves: store-split + lifecycle/update extract)
  |
  v
B2.5 (kt barrier: import swap + upgrade smoke)
  |
  v
B3 (parallel waves: install+uninstall, doctor+clean, then API)
  |
  v
B4 (serial: kt cmd wires RegisterLifecycle)
  |
  v
B5 (parallel waves: example-cli, docs, EN CANCHA matrix)
```

## Batch summary

| Batch | Description | Parallelism | Wall est |
|-------|-------------|-------------|----------|
| B0 | shipkit multi-module bootstrap (go.work, CI/release.yml, example skeleton, doc.go rewrite removing UI) | 1 task serial | ~45min |
| B1 | Extract base packages (frontmatter + lifecycle/migrations) | 2 tasks parallel | ~1h |
| B1.5 | kt barrier: import swap + smoke person/audit round-trip | 1 serial | ~30min |
| B2 | Extract store-split + lifecycle/update (2 waves due to inter-deps) | 2+2 parallel | ~2h |
| B2.5 | kt barrier: import swap + smoke `kt upgrade` round-trip | 1 serial | ~30min |
| B3 | NEW verbs TDD (install+uninstall parallel, doctor+clean parallel, then API serial) | 2+2+1 | ~2h |
| B4 | kt cmd wires `shipkit.RegisterLifecycle` for all 5 verbs | 1 serial | ~30min |
| B5 | example-cli + docs + EN CANCHA matrix on macos+ubuntu | 2+2 parallel waves | ~1h |

Total wall ~7.75h with parallelism (vs ~14h serial).

## Constraints (architect-coordinator must enforce)

- Sonnet 4.6 hard limit 200K tokens. Per task: inline prompt <= 40K tokens; everything else via engram topic_key with `mem_get_observation`.
- Max 4 distinct files MODIFIED per task.
- Max 3 tasks per batch launched in a single parallel Agent call.
- NEVER batch extract tasks with new-verb tasks in the same batch.
- 100% statement coverage strict per shipkit package (use sigstoreRealVerify pattern for network-calling functions: move to cmd layer or use SetVerifyCore adapter pattern with `ErrCosignNotConfigured` default).
- GOWORK=off in all CI environments.
- Conventional commits, NO Co-Authored-By, NO em-dashes anywhere.
- TDD red-green-refactor: test commit precedes implementation commit visibly in git history for NEW packages.
- Smoke commands: CONTENT assertions, not shape-only. Run the REAL entrypoint (binary), not internal funcs in test scope.
- Worktree isolation: each sub-Claude works in `/Users/fede/Projects/shipkit-wt-<batch>-<task>/` and `/Users/fede/Projects/kt-wt-<batch>-<task>/`. Merge back on success.
- **In-code documentation: MANDATORY**. Every exported identifier MUST have godoc, every package needs doc.go + README.md + Example tests, every multi-module package needs CHANGELOG.md. See engram `shipkit/standards/in-code-docs` (obs 5155) for the full spec. Sub-Claudes that fail the godoc gate return `status: failure: missing-docs`. NO merge until docs parity is reached. This OVERRIDES the project default "no comments" rule, per explicit user preference (obs 5154).

## Per-task template

The consolidated plan (engram `shipkit/plan/consolidated` Section 6) defines two task templates: one for extract tasks (B0, B1, B2) and one for NEW-verb tasks (B3). architect-coordinator builds Sonnet prompts from these templates per task, filling in: task ID, file scope, engram refs, acceptance criteria, after-work `mem_save` topic_key.

## Failure handling

Per consolidated plan Section 9 (engram `shipkit/plan/consolidated-tail`):
- Batch barrier failure: halt pipeline, save failure context to `shipkit/plan/batch-<N>-failure`, alert user, NO auto-retry.
- Sub-Claude 200K saturation: split task per Section 6 template, re-launch with smaller scope.
- Coverage gate failure: mandatory fix-first, NO merge until 100%.
- Tag push race: idempotent if commit matches, fail with mismatch otherwise.
- kt smoke regression: halt, surface failing assertion, human decides forward vs revert.
- proxy.golang.org indexing delay: poll 90s, then fail explicitly.

## EN CANCHA validation matrix

After each kt migration barrier (B1.5, B2.5, B4) and final cancha (B5): specific smoke commands with content assertions per engram `shipkit/plan/consolidated` Section 8. Final B5.d runs on macos-latest + ubuntu-latest GitHub Actions runners with example-cli built from shipkit, install.sh smoke, full lifecycle round-trip (install -> doctor -> update -> uninstall) with content-based assertions on stdout and disk state.

## Tag mapping at completion

shipkit tags created:
- frontmatter/v0.1.0
- lifecycle/migrations/v0.1.0
- store/v0.1.0
- lifecycle/update/v0.1.0
- lifecycle/install/v0.1.0
- lifecycle/uninstall/v0.1.0
- lifecycle/doctor/v0.1.0
- lifecycle/clean/v0.1.0
- ports/v0.1.0
- adapters/v0.1.0
- v0.1.0 (root module: Config + RegisterLifecycle)
- example-v0.1.0 (NOT released to tools)

kt tag created (post B5, human-triggered):
- v0.2.0

## Architect-coordinator handoff

This document is the entry point. To launch autonomous execution:
1. User invokes architect-coordinator skill ("acta como arquitecto coordinador" or "lanza la implementacion autonoma" or paste this MAESTRO).
2. Coordinator reads engram `shipkit/plan/consolidated` + `shipkit/plan/consolidated-tail` once at session start.
3. Coordinator decomposes batches per the dependency graph above, computes parallel tasks per M1, launches Sonnet sub-Claudes in worktrees.
4. Per batch: Sonnet executes, saves results to engram, returns Result Contract.
5. Coordinator merges worktrees on success, halts on failure, reports per-batch via PushNotification or stdout.

End state: kt v0.2.0 consumes shipkit packages with zero behavior change for kt's end users. shipkit v0.1.0 is a public reusable toolkit ready for the next personal CLI.
