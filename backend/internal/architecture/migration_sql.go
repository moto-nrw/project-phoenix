package architecture

import (
	"fmt"
	"strings"
)

// migrationSQLToken distinguishes quoted names and string values from syntax.
type migrationSQLToken struct {
	text            string
	quoted, literal bool
}

func (t migrationSQLToken) keyword(word string) bool {
	return !t.quoted && !t.literal && strings.EqualFold(t.text, word)
}

func migrationSQLTokens(query string) ([]migrationSQLToken, error) {
	var tokens []migrationSQLToken
	for i := 0; i < len(query); {
		switch {
		case strings.ContainsRune(" \t\r\n", rune(query[i])):
			i++
		case strings.HasPrefix(query[i:], "--"):
			end := strings.IndexByte(query[i:], '\n')
			if end < 0 {
				return tokens, nil
			}
			i += end
		case strings.HasPrefix(query[i:], "/*"):
			i += 2
			depth := 1
			for i < len(query) && depth > 0 {
				switch {
				case strings.HasPrefix(query[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(query[i:], "*/"):
					depth--
					i += 2
				default:
					i++
				}
			}
			if depth != 0 {
				return nil, fmt.Errorf("unterminated SQL comment")
			}
		case query[i] == '\'' || query[i] == '"':
			quote := query[i]
			i++
			var value strings.Builder
			closed := false
			for i < len(query) {
				if query[i] == quote {
					i++
					if i == len(query) || query[i] != quote {
						closed = true
						break
					}
				}
				value.WriteByte(query[i])
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated SQL quoted value")
			}
			tokens = append(tokens, migrationSQLToken{text: value.String(), quoted: quote == '"', literal: quote == '\''})
		case query[i] == '$' && sqlDollarQuotedLiteralEnd(query, i) > i:
			end := sqlDollarQuotedLiteralEnd(query, i)
			delimiterEnd := i + 1 + strings.IndexByte(query[i+1:], '$')
			delimiterLength := delimiterEnd - i + 1
			tokens = append(tokens, migrationSQLToken{text: query[delimiterEnd+1 : end-delimiterLength], literal: true})
			i = end
		case isSQLIdentifierStart(query[i]):
			start := i
			for i < len(query) && (isSQLIdentifierPart(query[i]) || query[i] == '$') {
				i++
			}
			tokens = append(tokens, migrationSQLToken{text: query[start:i]})
		default:
			tokens = append(tokens, migrationSQLToken{text: query[i : i+1]})
			i++
		}
	}
	return tokens, nil
}

func migrationSQLDataObjects(query string) (map[string]struct{}, error) {
	return migrationSQLBodyDataObjects(query, false)
}

func migrationSQLBodyDataObjects(query string, inBody bool) (map[string]struct{}, error) {
	tokens, err := migrationSQLTokens(query)
	if err != nil {
		return nil, err
	}
	// PostgreSQL joins newline-separated string literals before execution.
	// Joining all adjacent literal tokens is conservative for invalid SQL too.
	var joined []migrationSQLToken
	for _, tok := range tokens {
		if tok.literal && len(joined) > 0 && joined[len(joined)-1].literal {
			joined[len(joined)-1].text += tok.text
		} else {
			joined = append(joined, tok)
		}
	}
	tokens = joined
	objects := make(map[string]struct{})
	procedural := false
	function := false
	selectQuery := false
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.keyword(";") {
			procedural = false
			function = false
			selectQuery = false
		}
		if tok.keyword("INSERT") || tok.keyword("UPDATE") || tok.keyword("DELETE") || tok.keyword("MERGE") {
			selectQuery = false
		}
		if tok.keyword("SELECT") {
			selectQuery = true
		}
		if !inBody && selectQuery && tok.keyword("INTO") {
			next := i + 1
			if next < len(tokens) && (tokens[next].keyword("TEMP") || tokens[next].keyword("TEMPORARY")) {
				continue
			}
			if next < len(tokens) && tokens[next].keyword("UNLOGGED") {
				next++
			}
			if next < len(tokens) && tokens[next].keyword("TABLE") {
				next++
			}
			name, err := migrationObjectName(tokens, next)
			if err != nil {
				return nil, err
			}
			objects[name] = struct{}{}
			i = next + 2
			continue
		}
		if tok.keyword("DO") || (function && tok.keyword("AS")) {
			procedural = true
		}
		if inBody && tok.keyword("EXECUTE") {
			// Dynamic SQL cannot silently escape the ownership gate.
			if i+1 >= len(tokens) || !tokens[i+1].literal || (i+2 < len(tokens) && !tokens[i+2].keyword(";")) {
				return nil, fmt.Errorf("dynamic migration SQL cannot be classified; use static SQL")
			}
			procedural = true
		}
		if tok.literal && procedural {
			if i > 0 && (tokens[i-1].keyword("E") || tokens[i-1].keyword("&")) {
				return nil, fmt.Errorf("encoded executable SQL body cannot be classified; use dollar quoting")
			}
			procedural = false
			nested, err := migrationSQLBodyDataObjects(tok.text, true)
			if err != nil {
				return nil, err
			}
			for name := range nested {
				objects[name] = struct{}{}
			}
			continue
		}
		if !tok.keyword("CREATE") {
			continue
		}
		next := i + 1
		if next+1 < len(tokens) && tokens[next].keyword("OR") && tokens[next+1].keyword("REPLACE") {
			next += 2
		}
		if next >= len(tokens) {
			continue
		}
		if tokens[next].keyword("FUNCTION") || tokens[next].keyword("PROCEDURE") {
			function = true
			continue
		}
		// Session-local scratch objects are not persistent runtime data.
		if tokens[next].keyword("TEMP") || tokens[next].keyword("TEMPORARY") {
			continue
		}
		if tokens[next].keyword("UNLOGGED") || tokens[next].keyword("FOREIGN") || tokens[next].keyword("MATERIALIZED") {
			next++
		}
		if next >= len(tokens) || (!tokens[next].keyword("TABLE") && !tokens[next].keyword("VIEW") && !tokens[next].keyword("SEQUENCE")) {
			continue
		}
		next++
		if next+2 < len(tokens) && tokens[next].keyword("IF") && tokens[next+1].keyword("NOT") && tokens[next+2].keyword("EXISTS") {
			next += 3
		}
		name, err := migrationObjectName(tokens, next)
		if err != nil {
			return nil, err
		}
		objects[name] = struct{}{}
		i = next + 2
	}
	return objects, nil
}

func migrationObjectName(tokens []migrationSQLToken, next int) (string, error) {
	if next+2 >= len(tokens) || !tokens[next+1].keyword(".") || tokens[next].literal || tokens[next+2].literal {
		return "", fmt.Errorf("new writable data object must be schema-qualified")
	}
	name := migrationIdentifier(tokens[next]) + "." + migrationIdentifier(tokens[next+2])
	if !isDataObjectName(name) {
		return "", fmt.Errorf("new writable data object %q cannot be represented by the ownership policy", name)
	}
	return name, nil
}

func migrationIdentifier(tok migrationSQLToken) string {
	if tok.quoted {
		return tok.text
	}
	return strings.ToLower(tok.text)
}
