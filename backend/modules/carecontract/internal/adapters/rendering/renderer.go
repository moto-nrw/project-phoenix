// Package rendering binds the contract letter of the Betreuungsvertrag to
// the letter layout of Document Rendering (ADR 0121, ADR 0153). It
// translates the owner's values and stores nothing.
package rendering

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
)

// Renderer lays out a contract letter.
type Renderer struct {
	letters lists.LetterRenderer
}

var _ application.LetterRenderer = (*Renderer)(nil)

// NewRenderer returns the renderer over Document Rendering.
func NewRenderer() *Renderer { return &Renderer{letters: lists.NewLetterRenderer()} }

// RenderContract renders the contract letter as a PDF.
func (r *Renderer) RenderContract(letter application.Letter, filenameBase string) (carecontract.File, error) {
	blocks := make([]lists.LetterBlock, len(letter.Blocks))
	for i, block := range letter.Blocks {
		blocks[i] = lists.LetterBlock{Heading: block.Heading, Text: block.Text, Bold: block.Bold, Facts: facts(block.Facts)}
	}
	file, err := r.letters.RenderLetter(lists.Letter{
		Title: letter.Title, AccentColor: letter.AccentColor, Logo: letter.Logo, Sender: letter.Sender,
		Recipient: letter.Recipient, Facts: facts(letter.Facts), Subject: letter.Subject, Salutation: letter.Salutation,
		Blocks: blocks, Closing: letter.Closing, Footer: letter.Footer,
	}, filenameBase)
	if errors.Is(err, lists.ErrLetterheadTooLong) {
		return carecontract.File{}, fmt.Errorf("%w: %w", carecontract.ErrLetterheadTooLong, err)
	}
	if err != nil {
		return carecontract.File{}, err
	}
	return carecontract.File{Filename: file.Filename, ContentType: file.ContentType, Data: file.Data}, nil
}

func facts(values []application.LetterFact) []lists.LetterFact {
	translated := make([]lists.LetterFact, len(values))
	for i, value := range values {
		translated[i] = lists.LetterFact{Label: value.Label, Value: value.Value, Bold: value.Bold}
	}
	return translated
}
