// Package iban validates International Bank Account Numbers.
package iban

import (
	"errors"
	"fmt"
	"strings"
)

// Normalize removes spaces and upper-cases s.
func Normalize(s string) string {
	return strings.ToUpper(strings.ReplaceAll(s, " ", ""))
}

// Validate checks the format and the ISO 13616 mod 97 checksum of a
// normalized IBAN.
func Validate(s string) error {
	if len(s) < 15 || len(s) > 34 {
		return fmt.Errorf("IBAN %q: length %d is not between 15 and 34", s, len(s))
	}
	for i, r := range s {
		letter := r >= 'A' && r <= 'Z'
		digit := r >= '0' && r <= '9'
		switch {
		case i < 2 && !letter, i >= 2 && i < 4 && !digit, !letter && !digit:
			return fmt.Errorf("IBAN %q: unexpected character %q", s, r)
		}
	}
	rearranged := s[4:] + s[:4]
	rem := 0
	for _, r := range rearranged {
		if r >= 'A' {
			rem = (rem*100 + int(r-'A'+10)) % 97
		} else {
			rem = (rem*10 + int(r-'0')) % 97
		}
	}
	if rem != 1 {
		return errors.New("IBAN " + s + ": checksum mismatch")
	}
	return nil
}
