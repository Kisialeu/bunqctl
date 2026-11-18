// Package money handles amounts as integer cents, without floating point.
package money

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Cents is an amount in cents. It is negative for outgoing payments.
type Cents int64

var (
	inputRe = regexp.MustCompile(`^\d{1,12}\.\d{2}$`)
	apiRe   = regexp.MustCompile(`^-?\d{1,15}\.\d{2}$`)
)

// Parse parses a user-entered positive amount with two decimals, e.g. "12.34".
func Parse(s string) (Cents, error) {
	if !inputRe.MatchString(s) {
		return 0, fmt.Errorf("amount %q must look like 12.34", s)
	}
	c, err := parse(s)
	if err != nil {
		return 0, err
	}
	if c == 0 {
		return 0, fmt.Errorf("amount must be greater than 0.00")
	}
	return c, nil
}

// ParseAPI parses an amount returned by bunq, which may be negative.
func ParseAPI(s string) (Cents, error) {
	if !apiRe.MatchString(s) {
		return 0, fmt.Errorf("unexpected amount %q", s)
	}
	return parse(s)
}

func parse(s string) (Cents, error) {
	neg := strings.HasPrefix(s, "-")
	whole, frac, _ := strings.Cut(strings.TrimPrefix(s, "-"), ".")
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q: %w", s, err)
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("amount %q: %w", s, err)
	}
	c := Cents(w*100 + f)
	if neg {
		c = -c
	}
	return c, nil
}

// Abs returns the absolute value of c.
func (c Cents) Abs() Cents {
	if c < 0 {
		return -c
	}
	return c
}

// String formats c as "12.34" or "-12.34", the format bunq uses.
func (c Cents) String() string {
	sign := ""
	if c < 0 {
		sign = "-"
	}
	a := c.Abs()
	return fmt.Sprintf("%s%d.%02d", sign, a/100, a%100)
}
