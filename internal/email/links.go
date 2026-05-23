package email

import "strings"

// ExtractLinks scans the text and HTML bodies of msg for http(s) URLs and
// returns them in first-occurrence order, deduplicated case-sensitively.
// Text body is scanned before HTML so URLs that appear in both (typical for
// our duplicated <a href> + paragraph link templates) come from the text
// part first.
//
// This is the single source of truth for "what URLs does this rendered
// message contain" — both MockSender.LinkFor and FilesystemSender's
// .json sidecar route through it.
func ExtractLinks(msg Message) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, body := range [...]string{msg.TextBody, msg.HTMLBody} {
		for _, u := range urlsIn(body) {
			if _, dup := seen[u]; dup {
				continue
			}
			seen[u] = struct{}{}
			out = append(out, u)
		}
	}
	return out
}

// urlsIn returns every http(s) URL in body, in first-occurrence order.
// URLs terminate at whitespace or common HTML/markup delimiters
// (`"` `'` `<` `>` `)`).
func urlsIn(body string) []string {
	var out []string
	rest := body
	for {
		i := indexOfFirstScheme(rest)
		if i < 0 {
			return out
		}
		tail := rest[i:]
		end := len(tail)
		for j := 0; j < len(tail); j++ {
			c := tail[j]
			if c == ' ' || c == '\t' || c == '\r' || c == '\n' ||
				c == '"' || c == '\'' || c == '<' || c == '>' || c == ')' {
				end = j
				break
			}
		}
		out = append(out, tail[:end])
		rest = tail[end:]
	}
}

// indexOfFirstScheme returns the lowest index where "https://" or "http://"
// starts in s, or -1 if neither is present.
func indexOfFirstScheme(s string) int {
	a := strings.Index(s, "https://")
	b := strings.Index(s, "http://")
	switch {
	case a < 0:
		return b
	case b < 0:
		return a
	case a < b:
		return a
	default:
		return b
	}
}
