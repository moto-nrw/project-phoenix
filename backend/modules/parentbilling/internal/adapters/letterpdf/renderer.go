// Package letterpdf binds the letter renderer of the Elternabrechnung to the
// letter layout of Document Rendering (#4669, ADR 0121). It translates the
// owner's letter into the Document Rendering contract and stores nothing.
package letterpdf

import (
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/application"
)

// Renderer lays out the confirmation letters.
type Renderer struct {
	letters lists.LetterRenderer
}

var _ application.LetterRenderer = (*Renderer)(nil)

// NewRenderer returns the renderer over Document Rendering.
func NewRenderer() *Renderer {
	return &Renderer{letters: lists.NewLetterRenderer()}
}

// RenderLetter renders one letter as a PDF.
func (r *Renderer) RenderLetter(letter application.Letter, filenameBase string) (parentbilling.BillingLetterFile, error) {
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
		return parentbilling.BillingLetterFile{}, fmt.Errorf("%w: %w", parentbilling.ErrLetterheadTooLong, err)
	}
	if err != nil {
		return parentbilling.BillingLetterFile{}, err
	}
	return parentbilling.BillingLetterFile{Filename: file.Filename, ContentType: file.ContentType, Data: file.Data}, nil
}

func facts(values []application.LetterFact) []lists.LetterFact {
	translated := make([]lists.LetterFact, len(values))
	for i, value := range values {
		translated[i] = lists.LetterFact{Label: value.Label, Value: value.Value, Bold: value.Bold}
	}
	return translated
}
