package collation

// UnaccentLikePatternSQL is the SQL expression for a LIKE pattern of the
// search text bound to its ?: folded by public.f_unaccent, the database twin
// of FoldForSearch, then with its LIKE metacharacters escaped for ESCAPE '\'.
// The order matters: unaccent turns fullwidth ％, ＿ and ＼ into %, _ and \,
// so escaping first would let "％" match every name (#5791). Wrap it in
// '%' || … || '%' for a substring search.
const UnaccentLikePatternSQL = `replace(replace(replace(public.f_unaccent(?), '\', '\\'), '%', '\%'), '_', '\_')`
