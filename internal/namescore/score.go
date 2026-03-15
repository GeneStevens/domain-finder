package namescore

import (
	"fmt"
	"strings"
)

const (
	QualityNormal = "normal"
	QualityStrict = "strict"
)

type Score struct {
	Phonetic   int
	Structural int
	Brand      int
	Total      int
}

var (
	preferredEndings = []string{"ix", "on", "ar", "um", "us", "ra", "tra", "ex"}
	startupEndings   = []string{"ly", "fy", "ify", "ster"}
	techFragments    = []string{"data", "cloud", "stack", "code", "dev", "sync"}
	brandAnchors     = []string{"axis", "vector", "vect", "vertex", "vert", "crux", "forge", "fort", "iron", "atlas", "helix"}
	consumerPatterns = []string{"app", "buddy", "snap", "gram", "hub", "fy", "ify", "ly"}
)

func Evaluate(name string) Score {
	stem := strings.ToLower(strings.TrimSpace(name))
	if stem == "" {
		return Score{}
	}

	phonetic := phoneticScore(stem)
	structural := structuralScore(stem)
	brand := brandScore(stem)

	total := clamp(phonetic, 0, 50) + clamp(structural, 0, 30) + clamp(brand, 0, 20)
	return Score{
		Phonetic:   clamp(phonetic, 0, 50),
		Structural: clamp(structural, 0, 30),
		Brand:      clamp(brand, 0, 20),
		Total:      clamp(total, 0, 100),
	}
}

func NormalizeQuality(value string) (string, error) {
	switch normalized := strings.ToLower(strings.TrimSpace(value)); normalized {
	case "", QualityNormal:
		return QualityNormal, nil
	case QualityStrict:
		return QualityStrict, nil
	default:
		return "", fmt.Errorf("unsupported phonetic quality %q: want normal or strict", value)
	}
}

func EffectiveMinScore(minScore int, quality string) int {
	threshold := minScore
	if normalized, _ := NormalizeQuality(quality); normalized == QualityStrict && threshold < 70 {
		threshold = 70
	}
	if threshold < 0 {
		return 0
	}
	return threshold
}

func phoneticScore(stem string) int {
	if vowelGroupCount(stem) == 0 {
		return 0
	}
	if vowelGroupCount(stem) > 4 {
		return 0
	}
	if len(stem) >= 8 && countVowels(stem)*10 > len(stem)*6 {
		return 0
	}
	maxCluster := maxConsonantCluster(stem)
	if maxCluster >= 4 {
		return 0
	}
	if longestSpanWithoutVowel(stem) > 3 {
		return 0
	}

	score := 12
	switch syllables := vowelGroupCount(stem); {
	case syllables == 1:
		score += 8
	case syllables <= 3:
		score += 12
	default:
		score += 4
	}
	if maxCluster == 3 {
		score -= 25
	} else {
		score += 8
	}
	if startsWithRepeatedConsonants(stem) {
		score -= 22
	}
	score += 2 * countDistinctStrongConsonants(stem)
	if hasPreferredEnding(stem) {
		score += 8
	}
	return score
}

func structuralScore(stem string) int {
	length := len(stem)
	switch {
	case length >= 6 && length <= 10:
		// best scoring range
	case length >= 4 && length <= 12:
		// acceptable
	default:
		return 0
	}
	if hasAnySuffix(stem, startupEndings) {
		return 0
	}
	if hasAnyFragment(stem, techFragments) {
		return 0
	}

	score := 0
	if length >= 6 && length <= 10 {
		score += 20
	} else {
		score += 12
	}
	if startsWithConsonant(stem) {
		score += 4
	}
	if endsWithConsonant(stem) {
		score += 4
	}
	if maxConsonantCluster(stem) <= 2 {
		score += 2
	}
	return score
}

func brandScore(stem string) int {
	score := 0
	if hasAnyFragment(stem, brandAnchors) {
		score += 12
	}
	if hasPreferredEnding(stem) {
		score += 4
	}
	if startsWithConsonant(stem) && endsWithConsonant(stem) {
		score += 2
	}
	if hasAnyFragment(stem, consumerPatterns) {
		score -= 10
	}
	if strings.HasSuffix(stem, "y") {
		score -= 4
	}
	return score
}

func vowelGroupCount(stem string) int {
	count := 0
	inVowel := false
	for _, r := range stem {
		isVowel := strings.ContainsRune("aeiouy", r)
		if isVowel && !inVowel {
			count++
		}
		inVowel = isVowel
	}
	return count
}

func maxConsonantCluster(stem string) int {
	longest := 0
	current := 0
	for _, r := range stem {
		if strings.ContainsRune("aeiouy", r) {
			current = 0
			continue
		}
		if r < 'a' || r > 'z' {
			current = 0
			continue
		}
		current++
		if current > longest {
			longest = current
		}
	}
	return longest
}

func longestSpanWithoutVowel(stem string) int {
	longest := 0
	current := 0
	for _, r := range stem {
		if strings.ContainsRune("aeiouy", r) {
			current = 0
			continue
		}
		current++
		if current > longest {
			longest = current
		}
	}
	return longest
}

func countDistinctStrongConsonants(stem string) int {
	seen := make(map[rune]struct{})
	for _, r := range stem {
		if strings.ContainsRune("ktrxvd", r) {
			seen[r] = struct{}{}
		}
	}
	return len(seen)
}

func countVowels(stem string) int {
	count := 0
	for _, r := range stem {
		if strings.ContainsRune("aeiouy", r) {
			count++
		}
	}
	return count
}

func hasPreferredEnding(stem string) bool {
	return hasAnySuffix(stem, preferredEndings)
}

func hasAnySuffix(stem string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(stem, suffix) {
			return true
		}
	}
	return false
}

func hasAnyFragment(stem string, fragments []string) bool {
	for _, fragment := range fragments {
		if strings.Contains(stem, fragment) {
			return true
		}
	}
	return false
}

func startsWithRepeatedConsonants(stem string) bool {
	if len(stem) < 2 {
		return false
	}
	a := rune(stem[0])
	b := rune(stem[1])
	if a != b {
		return false
	}
	return !strings.ContainsRune("aeiouy", a)
}

func startsWithConsonant(stem string) bool {
	if stem == "" {
		return false
	}
	return !strings.ContainsRune("aeiouy", rune(stem[0]))
}

func endsWithConsonant(stem string) bool {
	if stem == "" {
		return false
	}
	return !strings.ContainsRune("aeiouy", rune(stem[len(stem)-1]))
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}
