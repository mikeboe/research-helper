package splitter

import (
	"github.com/tmc/langchaingo/textsplitter"
)

// TextSplitter wraps the langchaingo text splitter
type TextSplitter struct {
	splitter textsplitter.TextSplitter
}

// NewRecursiveCharacterTextSplitter creates a new recursive character text splitter
func NewRecursiveCharacterTextSplitter(chunkSize, chunkOverlap int) *TextSplitter {
	ts := textsplitter.NewRecursiveCharacter(
		textsplitter.WithChunkSize(chunkSize),
		textsplitter.WithChunkOverlap(chunkOverlap),
	)

	return &TextSplitter{splitter: ts}
}

// SplitText splits text into chunks
func (ts *TextSplitter) SplitText(text string) ([]string, error) {
	return ts.splitter.SplitText(text)
}
