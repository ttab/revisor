package revisor

import (
	"context"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/ttab/newsdoc"
)

type pruneStatus int

const (
	pruneOK       pruneStatus = iota // Valid or fixed.
	pruneRemoveMe                    // Caller should remove this block.
)

// Prune modifies the document to remove non-conforming parts where possible,
// and returns errors for things that cannot be fixed. Blocks that fail
// validation are removed if their count constraints allow it; otherwise the
// errors cascade up to the nearest removable ancestor, or are reported at the
// document root.
func (v *Validator) Prune(
	ctx context.Context, document *newsdoc.Document,
) ([]ValidationResult, error) {
	var res []ValidationResult

	var (
		blockConstraints     []BlockConstraintSet
		attributeConstraints []ConstraintMap
	)

	var declared bool

	vCtx := ValidationContext{
		coll:         ValueDiscarder{},
		ValidateHTML: v.validateHTML,
		ValidateEnum: v.enums.ValidValue,
	}

	_, err := uuid.Parse(document.UUID)
	if err != nil {
		res = append(res, ValidationResult{
			Entity: []EntityRef{{
				RefType: RefTypeAttribute,
				Name:    "uuid",
			}},
			Error: fmt.Sprintf("not a valid UUID: %v", err),
		})
	}

	for i := range v.documents {
		match := v.documents[i].Matches(document, &vCtx)
		if match == NoMatch {
			continue
		}

		if match == MatchDeclaration {
			declared = true
		}

		blockConstraints = append(blockConstraints, v.documents[i])
		attributeConstraints = append(
			attributeConstraints, v.documents[i].Attributes)
	}

	if !declared {
		res = append(res, ValidationResult{
			Error: fmt.Sprintf(
				"undeclared document type %q", document.Type),
		})
	}

	res = append(res, pruneDocumentAttributes(
		attributeConstraints, document, &vCtx)...)

	for _, kind := range blockKinds {
		blocks := getDocumentBlocks(document, kind)

		status, pruned, errs, err := v.pruneBlockSlice(
			ctx, document, blocks, kind,
			blockConstraints, vCtx, true,
		)
		if err != nil {
			return nil, err
		}

		setDocumentBlocks(document, kind, pruned)
		res = append(res, errs...)

		// At document level pruneRemoveMe should not happen because
		// pruneBlockSlice with documentLevel=true reports errors instead
		// of cascading. This is just defensive.
		_ = status
	}

	return res, nil
}

// getDocumentBlocks returns the block slice for the given kind from a document.
func getDocumentBlocks(doc *newsdoc.Document, kind BlockKind) []newsdoc.Block {
	switch kind {
	case BlockKindLink:
		return doc.Links
	case BlockKindMeta:
		return doc.Meta
	case BlockKindContent:
		return doc.Content
	}

	return nil
}

// setDocumentBlocks sets the block slice for the given kind on a document.
func setDocumentBlocks(doc *newsdoc.Document, kind BlockKind, blocks []newsdoc.Block) {
	switch kind {
	case BlockKindLink:
		doc.Links = blocks
	case BlockKindMeta:
		doc.Meta = blocks
	case BlockKindContent:
		doc.Content = blocks
	}
}

// getNestedBlocks returns the block slice for the given kind from a block.
func getNestedBlocks(block *newsdoc.Block, kind BlockKind) []newsdoc.Block {
	switch kind {
	case BlockKindLink:
		return block.Links
	case BlockKindMeta:
		return block.Meta
	case BlockKindContent:
		return block.Content
	}

	return nil
}

// setNestedBlocks sets the block slice for the given kind on a block.
func setNestedBlocks(block *newsdoc.Block, kind BlockKind, blocks []newsdoc.Block) {
	switch kind {
	case BlockKindLink:
		block.Links = blocks
	case BlockKindMeta:
		block.Meta = blocks
	case BlockKindContent:
		block.Content = blocks
	}
}

// blockRemovalAllowed checks whether removing one block matching this
// constraint is safe given the remaining count after removal.
func blockRemovalAllowed(
	constraint *BlockConstraint, remainingAfterRemoval int,
) bool {
	if constraint.Count != nil &&
		remainingAfterRemoval < *constraint.Count {
		return false
	}

	if constraint.MinCount != nil &&
		remainingAfterRemoval < *constraint.MinCount {
		return false
	}

	return true
}

// blockMatchInfo tracks which constraints matched a block during pruning.
type blockMatchInfo struct {
	defined                bool
	matchedConstraints     []BlockConstraintSet
	matchedAttrConstraints []ConstraintMap
	matchedDataConstraints []ConstraintMap
	matchedBlockPtrs       []*BlockConstraint
	declaredAttributes     map[blockAttributeKey]bool
}

// pruneBlockSlice prunes a slice of blocks, removing invalid ones where count
// constraints allow it.
func (v *Validator) pruneBlockSlice(
	ctx context.Context, doc *newsdoc.Document,
	blocks []newsdoc.Block, kind BlockKind,
	constraintSets []BlockConstraintSet,
	vCtx ValidationContext,
	documentLevel bool,
) (pruneStatus, []newsdoc.Block, []ValidationResult, error) {
	if len(blocks) == 0 {
		return pruneOK, blocks, nil, nil
	}

	// Phase 1: Match all blocks against constraints.
	matchInfos := make([]blockMatchInfo, len(blocks))
	counts := make(map[*BlockConstraint]int)

	for i := range blocks {
		matchInfos[i] = matchBlock(
			&blocks[i], kind, constraintSets, counts)
	}

	// Phase 2: Prune each block, marking for removal if needed.
	type removalCandidate struct {
		index      int
		cascadeErr []ValidationResult
	}

	var (
		removals []removalCandidate
		res      []ValidationResult
	)

	for i := range blocks {
		if !matchInfos[i].defined {
			// Undeclared block → mark for removal.
			removals = append(removals, removalCandidate{
				index: i,
				cascadeErr: []ValidationResult{{
					Error: "undeclared block type or rel",
				}},
			})

			continue
		}

		status, errs, err := v.pruneBlock(
			ctx, doc, &blocks[i], vCtx,
			matchInfos[i].matchedConstraints,
			matchInfos[i].matchedAttrConstraints,
			matchInfos[i].matchedDataConstraints,
			matchInfos[i].declaredAttributes,
		)
		if err != nil {
			return pruneOK, blocks, nil, err
		}

		if status == pruneRemoveMe {
			removals = append(removals, removalCandidate{
				index:      i,
				cascadeErr: errs,
			})

			continue
		}

		res = append(res, errs...)
	}

	// Phase 3: Check if each removal is allowed by count constraints.
	// Build a map of how many we plan to remove per constraint.
	removalCountDelta := make(map[*BlockConstraint]int)

	for _, r := range removals {
		for _, ptr := range matchInfos[r.index].matchedBlockPtrs {
			removalCountDelta[ptr]++
		}
	}

	var (
		allowedRemovals   []int
		forbiddenRemovals []removalCandidate
	)

	for _, r := range removals {
		forbidden := false

		for _, ptr := range matchInfos[r.index].matchedBlockPtrs {
			remaining := counts[ptr] - removalCountDelta[ptr]
			if !blockRemovalAllowed(ptr, remaining) {
				forbidden = true

				break
			}
		}

		if forbidden {
			forbiddenRemovals = append(forbiddenRemovals, r)
		} else {
			allowedRemovals = append(allowedRemovals, r.index)
		}
	}

	// Phase 4: Handle forbidden removals.
	for _, r := range forbiddenRemovals {
		entity := EntityRef{
			RefType:   RefTypeBlock,
			Index:     r.index,
			BlockKind: kind,
			Type:      blocks[r.index].Type,
			Rel:       blocks[r.index].Rel,
		}

		if documentLevel {
			// At document level, report cascade errors with entity.
			for j := range r.cascadeErr {
				r.cascadeErr[j].Entity = append(
					r.cascadeErr[j].Entity, entity)
			}

			res = append(res, r.cascadeErr...)
		} else {
			// At nested level, propagate up.
			for j := range r.cascadeErr {
				r.cascadeErr[j].Entity = append(
					r.cascadeErr[j].Entity, entity)
			}

			return pruneRemoveMe, blocks, r.cascadeErr, nil
		}
	}

	// Phase 5: Execute allowed removals (backwards for index stability).
	slices.Sort(allowedRemovals)

	for i := len(allowedRemovals) - 1; i >= 0; i-- {
		blocks = slices.Delete(blocks, allowedRemovals[i], allowedRemovals[i]+1)
	}

	// Phase 5.5: Remove excess blocks that exceed Count or MaxCount,
	// keeping the first N allowed blocks per constraint.
	blocks = pruneExcessBlocks(blocks, kind, constraintSets)

	// Phase 6: Post-removal count check (recount from scratch after all
	// removals).
	finalCounts := countBlockMatches(blocks, kind, constraintSets)

	for i := range constraintSets {
		for _, constraint := range constraintSets[i].BlockConstraints(kind) {
			count := finalCounts[constraint]

			minOK := nilOrGTE(constraint.MinCount, count)
			exactOK := nilOrEqual(constraint.Count, count)

			if !minOK || !exactOK {
				errResult := ValidationResult{
					Error: constraint.DescribeCountConstraint(kind),
				}

				if documentLevel {
					res = append(res, errResult)
				} else {
					return pruneRemoveMe, blocks,
						[]ValidationResult{errResult}, nil
				}
			}
		}
	}

	return pruneOK, blocks, res, nil
}

// matchBlock matches a single block against constraint sets and populates
// counts.
func matchBlock(
	b *newsdoc.Block, kind BlockKind,
	constraintSets []BlockConstraintSet,
	counts map[*BlockConstraint]int,
) blockMatchInfo {
	info := blockMatchInfo{
		declaredAttributes: make(map[blockAttributeKey]bool),
	}

	for _, set := range constraintSets {
		constraints := set.BlockConstraints(kind)

		for _, constraint := range constraints {
			match, attributes := constraint.Matches(b)
			if match == NoMatch {
				continue
			}

			if match == MatchDeclaration {
				info.defined = true
			}

			for _, a := range attributes {
				info.declaredAttributes[blockAttributeKey(a)] = true
			}

			counts[constraint]++

			info.matchedBlockPtrs = append(
				info.matchedBlockPtrs, constraint)
			info.matchedConstraints = append(
				info.matchedConstraints, constraint)
			info.matchedAttrConstraints = append(
				info.matchedAttrConstraints, constraint.Attributes)
			info.matchedDataConstraints = append(
				info.matchedDataConstraints, constraint.Data)
		}
	}

	return info
}

// pruneExcessBlocks removes blocks that exceed a constraint's Count or
// MaxCount limit, keeping the first N matching blocks per constraint.
func pruneExcessBlocks(
	blocks []newsdoc.Block, kind BlockKind,
	constraintSets []BlockConstraintSet,
) []newsdoc.Block {
	toRemove := make(map[int]bool)

	for _, set := range constraintSets {
		for _, constraint := range set.BlockConstraints(kind) {
			limit := constraintMaxAllowed(constraint)
			if limit < 0 {
				continue
			}

			// Collect indices of non-removed blocks matching this
			// constraint.
			var matching []int

			for i := range blocks {
				if toRemove[i] {
					continue
				}

				match, _ := constraint.Matches(&blocks[i])
				if match != NoMatch {
					matching = append(matching, i)
				}
			}

			if len(matching) <= limit {
				continue
			}

			// Keep the first `limit` blocks, mark the rest for
			// removal.
			for j := limit; j < len(matching); j++ {
				toRemove[matching[j]] = true
			}
		}
	}

	if len(toRemove) == 0 {
		return blocks
	}

	var removals []int

	for idx := range toRemove {
		removals = append(removals, idx)
	}

	slices.Sort(removals)

	for i := len(removals) - 1; i >= 0; i-- {
		blocks = slices.Delete(blocks, removals[i], removals[i]+1)
	}

	return blocks
}

// constraintMaxAllowed returns the maximum number of blocks allowed by a
// constraint's Count/MaxCount, or -1 if there is no upper limit.
func constraintMaxAllowed(constraint *BlockConstraint) int {
	limit := -1

	if constraint.Count != nil {
		limit = *constraint.Count
	}

	if constraint.MaxCount != nil {
		if limit < 0 || *constraint.MaxCount < limit {
			limit = *constraint.MaxCount
		}
	}

	return limit
}

// countBlockMatches counts how many blocks in the slice match each constraint.
func countBlockMatches(
	blocks []newsdoc.Block, kind BlockKind,
	constraintSets []BlockConstraintSet,
) map[*BlockConstraint]int {
	counts := make(map[*BlockConstraint]int)

	for i := range blocks {
		for _, set := range constraintSets {
			for _, constraint := range set.BlockConstraints(kind) {
				match, _ := constraint.Matches(&blocks[i])
				if match != NoMatch {
					counts[constraint]++
				}
			}
		}
	}

	return counts
}

// pruneBlock prunes a single block's attributes, data, and child blocks.
func (v *Validator) pruneBlock(
	ctx context.Context, doc *newsdoc.Document,
	b *newsdoc.Block, vCtx ValidationContext,
	matchedConstraints []BlockConstraintSet,
	matchedAttrConstraints []ConstraintMap,
	matchedDataConstraints []ConstraintMap,
	declaredAttributes map[blockAttributeKey]bool,
) (pruneStatus, []ValidationResult, error) {
	var res []ValidationResult

	// Prune attributes.
	status, errs := pruneBlockAttributes(
		matchedAttrConstraints, b, &vCtx, declaredAttributes)
	if status == pruneRemoveMe {
		return pruneRemoveMe, errs, nil
	}

	res = append(res, errs...)

	// Prune data.
	status, errs = pruneBlockData(b, matchedDataConstraints, &vCtx)
	if status == pruneRemoveMe {
		return pruneRemoveMe, errs, nil
	}

	res = append(res, errs...)

	// Prune child blocks.
	for _, kind := range blockKinds {
		childBlocks := getNestedBlocks(b, kind)

		status, pruned, errs, err := v.pruneBlockSlice(
			ctx, doc, childBlocks, kind,
			matchedConstraints, vCtx, false,
		)
		if err != nil {
			return pruneOK, nil, err
		}

		setNestedBlocks(b, kind, pruned)

		if status == pruneRemoveMe {
			return pruneRemoveMe, errs, nil
		}

		res = append(res, errs...)
	}

	return pruneOK, res, nil
}

// pruneBlockAttributes prunes block attributes. Invalid attributes that can be
// cleared (AllowEmpty or Optional) are set to "". Undeclared non-empty
// attributes are cleared. If a required attribute is invalid and cannot be
// fixed, pruneRemoveMe is returned.
func pruneBlockAttributes(
	constraints []ConstraintMap, b *newsdoc.Block,
	vCtx *ValidationContext,
	declaredAttributes map[blockAttributeKey]bool,
) (pruneStatus, []ValidationResult) {
	var res []ValidationResult

	for i := range constraints {
		for _, k := range constraints[i].Keys {
			declaredAttributes[blockAttributeKey(k)] = true

			value, ok := blockAttribute(b, k)
			check := constraints[i].Constraints[k]

			// Mirror validation: optional attributes allow empty.
			check.AllowEmpty = check.AllowEmpty || check.Optional

			_, err := check.Validate(value, ok, vCtx)
			if err == nil {
				continue
			}

			ref := EntityRef{
				RefType: RefTypeAttribute,
				Name:    k,
			}

			if check.AllowEmpty || check.Optional {
				setBlockAttribute(b, k, "")

				continue
			}

			// Required attribute with invalid value → can't fix.
			return pruneRemoveMe, []ValidationResult{{
				Entity: []EntityRef{ref},
				Error:  err.Error(),
			}}
		}
	}

	// Clear undeclared non-empty attributes.
	for _, attr := range allBlockAttributes {
		if declaredAttributes[attr] {
			continue
		}

		value, ok := blockAttribute(b, string(attr))
		if ok && value != "" {
			setBlockAttribute(b, string(attr), "")
		}
	}

	return pruneOK, res
}

// pruneBlockData prunes the data map of a block. Unknown keys are deleted.
// Optional keys with invalid values are deleted. AllowEmpty keys with invalid
// values are set to "". Required keys with invalid values trigger
// pruneRemoveMe.
func pruneBlockData(
	b *newsdoc.Block, constraints []ConstraintMap,
	vCtx *ValidationContext,
) (pruneStatus, []ValidationResult) {
	var res []ValidationResult

	known := make(map[string]bool)

	for i := range constraints {
		for _, k := range constraints[i].Keys {
			check := constraints[i].Constraints[k]

			var (
				value string
				ok    bool
			)

			if b.Data != nil {
				value, ok = b.Data[k]
			}

			if ok {
				known[k] = true
			}

			ref := EntityRef{
				RefType: RefTypeData,
				Name:    k,
			}

			if !ok && !check.Optional {
				// Missing required data → can't fix.
				return pruneRemoveMe, []ValidationResult{{
					Entity: []EntityRef{ref},
					Error:  "missing required attribute",
				}}
			}

			if !ok {
				continue
			}

			_, err := check.Validate(value, true, vCtx)
			if err == nil {
				continue
			}

			if check.AllowEmpty {
				b.Data[k] = ""

				continue
			}

			if check.Optional {
				delete(b.Data, k)

				continue
			}

			// Required data with invalid value → can't fix.
			return pruneRemoveMe, []ValidationResult{{
				Entity: []EntityRef{ref},
				Error:  err.Error(),
			}}
		}
	}

	// Delete unknown keys.
	for k := range b.Data {
		if !known[k] {
			delete(b.Data, k)
		}
	}

	if len(b.Data) == 0 {
		b.Data = nil
	}

	return pruneOK, res
}

// pruneDocumentAttributes prunes document-level attributes. Invalid values
// that can be cleared are set to "". Unfixable errors are reported directly
// since there is no cascade at document level.
func pruneDocumentAttributes(
	constraints []ConstraintMap, d *newsdoc.Document,
	vCtx *ValidationContext,
) []ValidationResult {
	var res []ValidationResult

	for i := range constraints {
		for _, k := range constraints[i].Keys {
			value, ok := documentAttribute(d, k)
			check := constraints[i].Constraints[k]

			_, err := check.Validate(value, ok, vCtx)
			if err == nil {
				continue
			}

			ref := EntityRef{
				RefType: RefTypeAttribute,
				Name:    k,
			}

			if check.AllowEmpty || check.Optional {
				setDocumentAttribute(d, k, "")

				continue
			}

			res = append(res, ValidationResult{
				Entity: []EntityRef{ref},
				Error:  err.Error(),
			})
		}
	}

	return res
}
