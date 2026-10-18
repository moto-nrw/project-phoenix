package listexport

// Glyph fallback for the designed PDF renderers (#1568 review): Inter
// covers Latin/Greek/Cyrillic, but student and guardian names can be
// Arabic, Hebrew, CJK, … — and gopdf substitutes missing glyphs SILENTLY
// (SubsetFontObj.AddChars swallows ErrGlyphNotFound), so an entirely
// Arabic name used to render as a blank cell. Text is therefore split
// into runs by Inter's actual cmap coverage; uncovered runs draw in the
// embedded GNU Unifont (full BMP coverage), and runes neither font has
// (e.g. emoji outside the BMP) become U+FFFD instead of disappearing.
//
// Known limitations, accepted deliberately: gopdf performs no OpenType
// shaping and no BiDi reordering, so Arabic renders as isolated forms
// and RTL scripts in visual LTR order — legible and identifying, not
// typographically correct.

import (
	_ "embed"
	"fmt"

	"golang.org/x/image/font/sfnt"
)

//go:embed assets/fonts/Unifont-Regular.ttf
var unifontRegular []byte

const fallbackFontFamily = "moto-fallback"

// fontCoverage answers "does this font have a real glyph for r" from the
// font's cmap — the same table gopdf consults before silently substituting.
type fontCoverage struct {
	font *sfnt.Font
	buf  sfnt.Buffer
}

func newFontCoverage(ttf []byte) (*fontCoverage, error) {
	f, err := sfnt.Parse(ttf)
	if err != nil {
		return nil, fmt.Errorf("parse font for coverage: %w", err)
	}
	return &fontCoverage{font: f}, nil
}

func (c *fontCoverage) covers(r rune) bool {
	idx, err := c.font.GlyphIndex(&c.buf, r)
	return err == nil && idx != 0
}

// fontRun is a maximal substring drawn with one font.
type fontRun struct {
	text     string
	fallback bool
}

// splitFontRuns partitions s into primary-font and fallback-font runs.
// Runes neither font covers are replaced with U+FFFD (kept in the
// fallback run) so the reader sees that something was there.
func splitFontRuns(s string, primary, fallback *fontCoverage) []fontRun {
	runs := []fontRun{}
	cur := []rune{}
	curFallback := false
	flush := func() {
		if len(cur) > 0 {
			runs = append(runs, fontRun{text: string(cur), fallback: curFallback})
			cur = cur[:0]
		}
	}
	for _, r := range s {
		toFallback := false
		switch {
		case primary.covers(r):
			toFallback = false
		case fallback.covers(r):
			toFallback = true
		default:
			r = '�'
			toFallback = true
		}
		if toFallback != curFallback {
			flush()
			curFallback = toFallback
		}
		cur = append(cur, r)
	}
	flush()
	return runs
}
