# Changelog

All notable changes to this project after v1.0.0 are documented here. The
entries below are derived from release tags; see the linked PRs for full
detail.

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
