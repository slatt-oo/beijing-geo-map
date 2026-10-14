package repository

import "strings"

// PostgreSQL 中 LIKE/ILIKE 的默认转义字符是反斜杠。
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLike(s string) string {
	return likeEscaper.Replace(s)
}
