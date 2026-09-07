# Changelog

All notable changes to this project after v1.0.0 are documented here. The
entries below are derived from release tags; see the linked PRs for full
detail.

## [v1.0.3] - 2026-09-05

**Breaking (toolchain):** the `go` directive is now 1.26.5, up from 1.25.7, so
building against revisor needs Go 1.26.5 or later.

Changes:

- Fix the WebAssembly build, which had not compiled since `ValidateDocument`
  grew a second return value — `cmd/wasm` now surfaces a validation error to
  the JS caller as a rejected promise instead of dropping it. CI builds and
  lints the `js/wasm` target so it cannot rot again unnoticed.
- Regenerate `spec.schema.json`, which was missing `description` on
  `Deprecation` and `colourFormats` on string constraints. Both were already
  accepted by the parser, so specs using them validated against revisor while
  failing against the published schema.
- The `revisor` and `serve-wasm` commands moved from urfave/cli v2 to v3.
  `serve-wasm`'s `--dir` is now a plain string flag rather than a path flag;
  it takes the same values.
- Dependency upgrades: gobwas/glob to v1.0.0 (a rewritten matching engine,
  verified to produce identical results for every glob pattern in the bundled
  core and tt schemas), newsdoc to v1.1.0, invopop/jsonschema to v0.14.0, and
  golang.org/x/net to v0.58.0.

## [v1.0.2] - 2026-07-31

**Behaviour change (prune):** `Prune` now reports `count`/`minCount` violations
for block collections that are empty or absent, which it previously skipped
entirely. Consumers that treat leftover prune results as "this document doesn't
satisfy the schemas" will start rejecting documents that are missing a required
block, and blocks that are missing a required nested collection are now removed
by the prune cascade. Both were silently accepted before.

- Fix `pruneBlockSlice` returning early for an empty block slice, which skipped
  the post-removal count check. A document missing a required block collection
  outright passed prune clean while `ValidateDocument` rejected it, breaking
  prune's contract that what it hands back either satisfies the constraints or
  comes with results explaining what it couldn't resolve. The matching and
  removal phases are still skipped for an empty collection, and the block slice
  is returned untouched so that absent collections aren't materialised as empty
  ones in the pruned document.

## [v1.0.1] - 2026-07-29

- Add a `WithOptionalDocumentUUID()` validation option for document types that
  are identified by something other than a UUID and never get one assigned, so
  that an empty UUID stops being reported as a validation error. A UUID that is
  set still has to be a valid one. (#11)

## [v1.0.0] - 2026-04-21

- initial 1.0 release
