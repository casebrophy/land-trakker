package parser

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	acresPattern = regexp.MustCompile(`(?i)~?±?[\s]*([0-9,]+(?:\.[0-9]+)?)\s*-?(?:acres?|ac\b)`)
	pricePattern = regexp.MustCompile(`\$\s*([0-9,]+(?:\.[0-9]+)?)\s*([kmKM])?`)
)

func ParseAcres(s string) (float64, bool) {
	matches := acresPattern.FindStringSubmatch(s)
	if matches == nil {
		return 0, false
	}
	numStr := strings.ReplaceAll(matches[1], ",", "")
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil || val <= 0 {
		return 0, false
	}
	return val, true
}

func ParsePrice(s string) (int64, bool) {
	matches := pricePattern.FindStringSubmatch(s)
	if matches == nil {
		return 0, false
	}
	numStr := strings.ReplaceAll(matches[1], ",", "")
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil || val <= 0 {
		return 0, false
	}

	multiplier := float64(1)
	if len(matches) > 2 && matches[2] != "" {
		if strings.ToLower(matches[2]) == "k" {
			multiplier = 1000
		} else if strings.ToLower(matches[2]) == "m" {
			multiplier = 1000000
		}
	}

	result := int64(val * multiplier)
	return result, true
}

func NormalizeAddress(s string) string {
	s = strings.TrimSpace(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")

	parts := strings.Split(s, ",")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		part = strings.TrimSuffix(part, ".")
		parts[i] = part
	}

	return strings.Join(parts, ", ")
}
