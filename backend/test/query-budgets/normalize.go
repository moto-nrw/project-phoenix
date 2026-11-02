package querybudgets

import (
	"regexp"
	"strings"
)

var sqlNumber = regexp.MustCompile(`^(?:0[xX][0-9a-fA-F_]+|0[oO][0-7_]+|0[bB][01_]+|(?:[0-9][0-9_]*(?:\.[0-9_]*)?|\.[0-9][0-9_]*)(?:[eE][+-]?[0-9][0-9_]*)?)`)
var dollarQuote = regexp.MustCompile(`^\$(?:[a-zA-Z_][a-zA-Z_0-9]*)?\$`)

// This is lexical normalization, not a SQL parser. Quoted identifiers remain
// byte-exact; PostgreSQL string, dollar and numeric literals become ?. Comments
// are discarded. Token boundaries prevent literals from absorbing identifiers.
func normalizeSQL(sql string) string {
	var tokens []string
	for i := 0; i < len(sql); {
		if end := sqlCommentEnd(sql, i); end > i {
			i = end
			continue
		}
		if strings.ContainsRune(" \t\r\n\f", rune(sql[i])) {
			i++
			continue
		}
		token, end := sqlToken(sql, i)
		tokens = append(tokens, token)
		i = end
	}
	return strings.Join(tokens, " ")
}

func sqlToken(sql string, i int) (string, int) {
	if sql[i] == '"' {
		end := quotedEnd(sql, i, '"', false)
		return sql[i:end], end
	}
	if start, escapes := stringQuoteStart(sql, i); start >= i {
		return "?", quotedEnd(sql, start, '\'', escapes)
	}
	if sql[i] == '$' {
		if tag := dollarQuote.FindString(sql[i:]); tag != "" {
			if end := strings.Index(sql[i+len(tag):], tag); end >= 0 {
				return "?", i + len(tag)*2 + end
			}
			return "?", len(sql)
		}
	}
	if number := sqlNumber.FindString(sql[i:]); number != "" {
		return "?", i + len(number)
	}
	if sqlIdentStart(sql[i]) {
		end := i + 1
		for end < len(sql) && sqlIdentContinue(sql[end]) {
			end++
		}
		identifier := strings.ToLower(sql[i:end])
		switch identifier {
		case "true", "false", "null":
			return "?", end
		default:
			return identifier, end
		}
	}
	return sql[i : i+1], i + 1
}

func stringQuoteStart(sql string, i int) (int, bool) {
	if sql[i] == '\'' {
		return i, false
	}
	if i+1 < len(sql) && sql[i+1] == '\'' && strings.ContainsRune("eEbBxXnN", rune(sql[i])) {
		return i + 1, sql[i] == 'e' || sql[i] == 'E'
	}
	if i+2 < len(sql) && (sql[i] == 'u' || sql[i] == 'U') && sql[i+1:i+3] == "&'" {
		return i + 2, false
	}
	return -1, false
}

func quotedEnd(sql string, start int, quote byte, escapes bool) int {
	for i := start + 1; i < len(sql); i++ {
		if escapes && sql[i] == '\\' {
			i++
		} else if sql[i] == quote {
			if i+1 < len(sql) && sql[i+1] == quote {
				i++
				continue
			}
			return i + 1
		}
	}
	return len(sql)
}

func sqlCommentEnd(sql string, start int) int {
	if strings.HasPrefix(sql[start:], "--") {
		if end := strings.IndexByte(sql[start:], '\n'); end >= 0 {
			return start + end + 1
		}
		return len(sql)
	}
	if !strings.HasPrefix(sql[start:], "/*") {
		return start
	}
	depth := 1
	for i := start + 2; i < len(sql)-1; i++ {
		switch sql[i : i+2] {
		case "/*":
			depth++
			i++
		case "*/":
			depth--
			i++
			if depth == 0 {
				return i + 1
			}
		}
	}
	return len(sql)
}

func sqlIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 128
}

func sqlIdentContinue(c byte) bool {
	return sqlIdentStart(c) || c >= '0' && c <= '9' || c == '$'
}
