package bunq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"
)

// Time is a bunq timestamp. bunq does not document the format; it is read
// as "2006-01-02 15:04:05.000000" in UTC, with RFC 3339 as a fallback.
type Time struct{ time.Time }

const timeLayout = "2006-01-02 15:04:05.000000"

// UnmarshalJSON parses a bunq timestamp string.
func (t *Time) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s == "" {
		return nil
	}
	v, err := ParseTime(s)
	t.Time = v
	return err
}

// ParseTime parses a bunq timestamp string as UTC.
func ParseTime(s string) (time.Time, error) {
	for _, layout := range []string{timeLayout, "2006-01-02 15:04:05", time.RFC3339Nano} {
		if v, err := time.ParseInLocation(layout, s, time.UTC); err == nil {
			return v, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown time format %q", s)
}

// Amount is a money amount. Value is a decimal string with two places.
type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// Pointer identifies a counterparty or an account alias.
type Pointer struct {
	Type  string `json:"type"` // IBAN, EMAIL or PHONE_NUMBER
	Value string `json:"value"`
	Name  string `json:"name,omitempty"`
}

// Pointers decodes from either a single Pointer or a list of them; bunq
// documents the account alias as an object but returns a list.
type Pointers []Pointer

// UnmarshalJSON accepts an object or an array.
func (p *Pointers) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '{' {
		var one Pointer
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		*p = Pointers{one}
		return nil
	}
	var many []Pointer
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*p = many
	return nil
}

// Value returns the value of the first pointer of type typ, or "".
func (p Pointers) Value(typ string) string {
	for _, ptr := range p {
		if ptr.Type == typ {
			return ptr.Value
		}
	}
	return ""
}

// Int decodes from a JSON number or a numeric string; bunq uses both.
type Int int

// UnmarshalJSON accepts 12, "12" and null.
func (n *Int) UnmarshalJSON(b []byte) error {
	b = bytes.Trim(b, `"`)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	var v int
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*n = Int(v)
	return nil
}

// LabelMonetaryAccount is the public information of a payment party.
type LabelMonetaryAccount struct {
	IBAN        string `json:"iban"`
	DisplayName string `json:"display_name"`
}
