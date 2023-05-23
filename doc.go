package revisor

import "github.com/ttab/newsdoc"

// BlockSource acts as an intermediary to allow us to write code that can treat
// both documents and blocks as a source of blocks.
type BlockSource interface {
	// GetBlocks returns the child blocks of the specified type.
	GetBlocks(kind BlockKind) []newsdoc.Block
}

func NewDocumentBlocks(document *newsdoc.Document) DocumentBlocks {
	return DocumentBlocks{
		doc: document,
	}
}

type DocumentBlocks struct {
	doc *newsdoc.Document
}

func (db DocumentBlocks) GetBlocks(kind BlockKind) []newsdoc.Block {
	switch kind {
	case BlockKindLink:
		return db.doc.Links
	case BlockKindMeta:
		return db.doc.Meta
	case BlockKindContent:
		return db.doc.Content
	}

	return nil
}

func NewNestedBlocks(block *newsdoc.Block) NestedBlocks {
	return NestedBlocks{
		block: block,
	}
}

type NestedBlocks struct {
	block *newsdoc.Block
}

func (nb NestedBlocks) GetBlocks(kind BlockKind) []newsdoc.Block {
	switch kind {
	case BlockKindLink:
		return nb.block.Links
	case BlockKindMeta:
		return nb.block.Meta
	case BlockKindContent:
		return nb.block.Content
	}

	return nil
}
