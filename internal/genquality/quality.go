package genquality

import (
	"fmt"
	"strings"
)

const (
	ProfileIndustrial = "industrial"
	ProfileOff        = "off"
)

type Reason string

const (
	ReasonPharmaLikeSuffix   Reason = "pharma_like_suffix"
	ReasonSoftOpenEnding     Reason = "soft_open_ending"
	ReasonVowelHeavy         Reason = "vowel_heavy"
	ReasonMushyVowelFlow     Reason = "mushy_vowel_flow"
	ReasonCVAlternationMush  Reason = "cv_alternation_mush"
	ReasonWeakConsonantShape Reason = "weak_consonant_shape"
)

// Evaluation describes one generated-stem quality decision.
type Evaluation struct {
	Profile  string
	Accepted bool
	Score    int
	Reasons  []Reason
}

// NormalizeProfile canonicalizes supported quality-profile names.
func NormalizeProfile(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return ProfileOff, nil
	case ProfileIndustrial:
		return ProfileIndustrial, nil
	case ProfileOff, "none", "disabled":
		return ProfileOff, nil
	default:
		return "", fmt.Errorf("unsupported generate quality profile %q: want industrial or off", value)
	}
}

// Evaluate applies the selected quality profile to one generated stem.
func Evaluate(stem, profile string) Evaluation {
	normalized, err := NormalizeProfile(profile)
	if err != nil {
		return Evaluation{Profile: profile, Accepted: false, Reasons: []Reason{ReasonWeakConsonantShape}}
	}
	if normalized == ProfileOff {
		return Evaluation{Profile: normalized, Accepted: true}
	}
	return evaluateIndustrial(strings.ToLower(strings.TrimSpace(stem)))
}

func evaluateIndustrial(stem string) Evaluation {
	score := 0
	reasons := make([]Reason, 0, 4)
	addReason := func(reason Reason, delta int) {
		score += delta
		for _, existing := range reasons {
			if existing == reason {
				return
			}
		}
		reasons = append(reasons, reason)
	}

	length := len(stem)
	switch {
	case length >= 5 && length <= 7:
		score += 3
	case length == 8:
		score += 2
	case length <= 10:
		score++
	}

	if hasHardEnding(stem) {
		score += 3
	} else if hasSoftOpenEnding(stem) {
		addReason(ReasonSoftOpenEnding, -2)
	}

	if hasConsonantAnchor(stem) {
		score += 2
	}
	if hasStrongConsonantDensity(stem) {
		score += 2
	}
	if hasHardStarter(stem) {
		score++
	}

	vowels, consonants := countLetters(stem)
	if consonants >= vowels+1 {
		score += 2
	}
	if length >= 6 && vowels*2 > length+1 {
		addReason(ReasonVowelHeavy, -2)
	}
	if hasMushyVowelFlow(stem) {
		addReason(ReasonMushyVowelFlow, -2)
	}
	if hasCVAlternationMush(stem) {
		addReason(ReasonCVAlternationMush, -2)
	}
	if hasPharmaLikeSuffix(stem) {
		addReason(ReasonPharmaLikeSuffix, -4)
	}
	if !hasHardEnding(stem) && !hasConsonantAnchor(stem) && !hasStrongConsonantDensity(stem) && consonants <= vowels+1 {
		addReason(ReasonWeakConsonantShape, -3)
	}

	return Evaluation{
		Profile:  ProfileIndustrial,
		Accepted: score >= 1,
		Score:    score,
		Reasons:  reasons,
	}
}

func hasHardEnding(stem string) bool {
	if stem == "" {
		return false
	}
	switch stem[len(stem)-1] {
	case 'k', 't', 'x', 'q', 'r', 'd', 'p', 'm', 'n', 'g':
		return true
	default:
		return false
	}
}

func hasSoftOpenEnding(stem string) bool {
	if stem == "" {
		return false
	}
	switch stem[len(stem)-1] {
	case 'a', 'e', 'i', 'o', 'u', 'y':
		return true
	default:
		return false
	}
}

func hasConsonantAnchor(stem string) bool {
	anchors := []string{
		"str", "ctr", "tr", "dr", "cr", "gr", "rd", "rk", "sk", "xt", "nd", "pt", "lk", "nx", "kt", "st", "ft", "rk", "nt", "ld", "nz", "qx", "rx", "vr",
	}
	for _, anchor := range anchors {
		if strings.Contains(stem, anchor) {
			return true
		}
	}
	return false
}

func hasMushyVowelFlow(stem string) bool {
	if longestVowelRun(stem) >= 3 {
		return true
	}
	patterns := []string{"ia", "eo", "ua", "oa", "ae", "io", "ai", "ea", "oe"}
	for _, pattern := range patterns {
		if strings.Contains(stem, pattern) {
			return true
		}
	}
	return false
}

func hasPharmaLikeSuffix(stem string) bool {
	suffixes := []string{
		"zyme", "pharm", "cure", "thera", "gen", "med", "bio", "via", "viva", "vera", "lia", "ria", "nia", "ia", "ara", "ora", "iva", "eria", "oria",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(stem, suffix) {
			return true
		}
	}
	return false
}

func hasCVAlternationMush(stem string) bool {
	if len(stem) < 6 {
		return false
	}
	alternations := 0
	prevVowel := isVowel(rune(stem[0]))
	for _, r := range stem[1:] {
		currentVowel := isVowel(r)
		if currentVowel != prevVowel {
			alternations++
		}
		prevVowel = currentVowel
	}
	return alternations >= len(stem)-2 && !hasConsonantAnchor(stem)
}

func hasStrongConsonantDensity(stem string) bool {
	_, consonants := countLetters(stem)
	return consonants >= 4 && consonantRun(stem) >= 2
}

func hasHardStarter(stem string) bool {
	if stem == "" {
		return false
	}
	switch stem[0] {
	case 'k', 'q', 't', 'd', 'f', 'g', 'p', 'b', 'c', 'x':
		return true
	default:
		return false
	}
}

func countLetters(stem string) (vowels, consonants int) {
	for _, r := range stem {
		if strings.ContainsRune("aeiou", r) {
			vowels++
		} else if 'a' <= r && r <= 'z' {
			consonants++
		}
	}
	return vowels, consonants
}

func longestVowelRun(stem string) int {
	longest := 0
	current := 0
	for _, r := range stem {
		switch r {
		case 'a', 'e', 'i', 'o', 'u':
			current++
			if current > longest {
				longest = current
			}
		default:
			current = 0
		}
	}
	return longest
}

func consonantRun(stem string) int {
	longest := 0
	current := 0
	for _, r := range stem {
		if isVowel(r) {
			current = 0
			continue
		}
		if 'a' <= r && r <= 'z' {
			current++
			if current > longest {
				longest = current
			}
		}
	}
	return longest
}

func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	default:
		return false
	}
}
