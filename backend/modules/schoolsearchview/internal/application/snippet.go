package application

import (
	"strings"
	"unicode"

	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
	"github.com/moto-ogs/project-phoenix/sharedkernel/collation"
)

const ellipsis = '…'

// matchSnippet is the part of text around the first place query matches it,
// folded like the search (case and accents), at most
// schoolsearchview.MaxMatchTextRunes characters with "…" where it was cut and
// whole words at the cuts. A text that fits stays whole; one the folding
// cannot place (the database folds a little differently) shows its start.
func matchSnippet(text, query string) string {
	runes := []rune(strings.TrimSpace(text))
	if len(runes) <= schoolsearchview.MaxMatchTextRunes {
		return string(runes)
	}
	budget := schoolsearchview.MaxMatchTextRunes - 2
	start, end, ok := foldedMatch(runes, query)
	if !ok {
		start, end = 0, 0
	}
	if end-start >= budget {
		end = start + budget
	}
	from := max(0, start-(budget-(end-start))/2)
	to := min(len(runes), from+budget)
	from = max(0, to-budget)
	if from > 0 {
		if space := indexSpace(runes[from:start]); space >= 0 {
			from += space + 1
		}
	}
	if to < len(runes) {
		if space := lastIndexSpace(runes[end:to]); space >= 0 {
			to = end + space
		}
	}
	snippet := strings.TrimSpace(string(runes[from:to]))
	if from > 0 {
		snippet = string(ellipsis) + snippet
	}
	if to < len(runes) {
		snippet += string(ellipsis)
	}
	return snippet
}

// foldedMatch finds query in runes the way the search compares, and returns
// the matched span in runes of the original text.
func foldedMatch(runes []rune, query string) (int, int, bool) {
	needle := []rune(collation.FoldForSearch(strings.TrimSpace(query)))
	if len(needle) == 0 {
		return 0, 0, false
	}
	folded := make([]rune, 0, len(runes))
	origin := make([]int, 0, len(runes))
	for index, r := range runes {
		for _, piece := range collation.FoldForSearch(string(r)) {
			folded = append(folded, piece)
			origin = append(origin, index)
		}
	}
	for at := 0; at+len(needle) <= len(folded); at++ {
		if string(folded[at:at+len(needle)]) == string(needle) {
			return origin[at], origin[at+len(needle)-1] + 1, true
		}
	}
	return 0, 0, false
}

func indexSpace(runes []rune) int {
	for i, r := range runes {
		if unicode.IsSpace(r) {
			return i
		}
	}
	return -1
}

func lastIndexSpace(runes []rune) int {
	for i := len(runes) - 1; i >= 0; i-- {
		if unicode.IsSpace(runes[i]) {
			return i
		}
	}
	return -1
}
