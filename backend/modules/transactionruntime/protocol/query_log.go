package protocol

// RedactLiteralsInLog opens a statement whose string literals carry what a
// person typed, such as the text of a search (ADR 0227). bun writes bound
// values into the SQL it hands to the query log, so the log replaces every
// string literal of a statement that starts with this marker before it
// writes the statement. The marker is an SQL comment: Postgres ignores it.
const RedactLiteralsInLog = "/* moto:redact-literals */ "
