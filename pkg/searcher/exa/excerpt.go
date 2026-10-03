package exa

import (
	"math"
	"slices"
	"strings"
	"unicode"
)

// Search returns evidence for discovery; the scraper remains the full-text path.
// All extraction here is deterministic and local. No text or query is sent to
// an Exa synthesis, highlights, summary, or research endpoint.
const excerptCharacters = 1400

func searchExcerpt(text, query string) string {
	chars := []rune(text)
	if len(chars) <= excerptCharacters {
		return text
	}
	words := strings.FieldsFunc(strings.ToLower(query), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	terms := make(map[string]bool)
	for _, word := range words {
		if len([]rune(word)) > 1 {
			terms[word] = true
		}
	}
	type passage struct {
		start, end int
		matches    []string
	}
	var passages []passage
	frequency := make(map[string]int)
	for start := 0; start < len(chars); start += 1000 {
		end := min(start+excerptCharacters, len(chars))
		lower := strings.ToLower(string(chars[start:end]))
		p := passage{start: start, end: end}
		for term := range terms {
			if strings.Contains(lower, term) {
				p.matches = append(p.matches, term)
				frequency[term]++
			}
		}
		slices.Sort(p.matches)
		passages = append(passages, p)
	}
	best, bestScore := passages[0], -1.0
	for _, p := range passages {
		score := 0.0
		for _, term := range p.matches {
			// Common navigation words contribute less than distinctive terms.
			score += math.Log(1 + float64(len(passages))/float64(frequency[term]))
		}
		if score > bestScore {
			best, bestScore = p, score
		}
	}
	return "[Excerpt; fetch the URL for full text]\n" + string(chars[best.start:best.end])
}
