package records

// Letter is one letter in the layout of its sender (#4669): the sender's
// logo, letterhead and colour, an address field, a block of facts beside
// it, subject, salutation, the body and a footer on every page. The
// consumer writes every text; the renderer only lays it out. A letter that
// does not fit one A4 page continues on the next.
type Letter struct {
	// Title names the PDF document.
	Title string
	// AccentColor is the sender's colour as #RRGGBB. Empty or unusable uses
	// the renderer's ink.
	AccentColor string
	// Logo is a PNG or JPEG image; nil prints none.
	Logo []byte
	// Sender is the letterhead, one line each. The first line is the
	// sender's name and the return line of the address field.
	Sender []string
	// Recipient is the address field, one line each.
	Recipient []string
	// Facts is the block beside the address field, such as number, date and
	// the person to ask.
	Facts      []LetterFact
	Subject    string
	Salutation string
	Blocks     []LetterBlock
	// Closing ends the letter, one line each, such as the greeting and the
	// sender's name.
	Closing []string
	// Footer is printed at the foot of every page, one line each.
	Footer []string
}

// LetterFact is a label with its value. Bold prints the line in the
// sender's weight, such as the total of a table.
type LetterFact struct {
	Label string
	Value string
	Bold  bool
}

// LetterBlock is a part of the body: a paragraph, then an optional
// two-column table. Heading is printed bold above the paragraph; Bold
// prints the paragraph itself bold. Line breaks in Text are kept.
type LetterBlock struct {
	Heading string
	Text    string
	Bold    bool
	Facts   []LetterFact
}
