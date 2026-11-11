// Package isbn validates and converts ISBN-10 and ISBN-13 codes.
package isbn

import (
	"regexp"
	"strings"
)

// Normalize strips hyphens and spaces from s and returns it when it is a
// valid ISBN-10 (with an uppercase X check digit) or ISBN-13.
func Normalize(s string) (string, bool) {
	s = strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(s))
	switch {
	case len(s) == 10 && valid10(s):
		return s, true
	case len(s) == 13 && valid13(s):
		return s, true
	}
	return "", false
}

// To13 returns the ISBN-13 form of a valid ISBN, or "" when s is not one.
func To13(s string) string {
	n, ok := Normalize(s)
	if !ok {
		return ""
	}
	if len(n) == 13 {
		return n
	}
	body := "978" + n[:9]
	return body + string(check13(body))
}

// To10 returns the ISBN-10 form of a valid ISBN, or "" when s is not one or
// is an ISBN-13 outside the 978 prefix (979 codes have no ISBN-10).
func To10(s string) string {
	n, ok := Normalize(s)
	if !ok {
		return ""
	}
	if len(n) == 10 {
		return n
	}
	if !strings.HasPrefix(n, "978") {
		return ""
	}
	body := n[3:12]
	return body + string(check10(body))
}

var digitRuns = regexp.MustCompile(`\d+`)

// Find returns the first valid ISBN-13 (978 or 979 prefix) that stands as a
// whole number in s, such as the file name in an artwork URL, or "".
func Find(s string) string {
	for _, run := range digitRuns.FindAllString(s, -1) {
		if len(run) == 13 && (strings.HasPrefix(run, "978") || strings.HasPrefix(run, "979")) && valid13(run) {
			return run
		}
	}
	return ""
}

func valid13(s string) bool {
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return check13(s[:12]) == s[12]
}

func valid10(s string) bool {
	for i := range 9 {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return check10(s[:9]) == s[9]
}

// check13 returns the check digit of a 12-digit ISBN-13 body.
func check13(body string) byte {
	sum := 0
	for i := range 12 {
		d := int(body[i] - '0')
		if i%2 == 1 {
			d *= 3
		}
		sum += d
	}
	return byte('0' + (10-sum%10)%10)
}

// check10 returns the check digit (0-9 or X) of a 9-digit ISBN-10 body.
func check10(body string) byte {
	sum := 0
	for i := range 9 {
		sum += (10 - i) * int(body[i]-'0')
	}
	c := (11 - sum%11) % 11
	if c == 10 {
		return 'X'
	}
	return byte('0' + c)
}
