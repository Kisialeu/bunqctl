package iban

import "testing"

func Test_Normalize(t *testing.T) {
	if got := Normalize("nl07 bunq 2108 2146 23"); got != "NL07BUNQ2108214623" {
		t.Errorf("Normalize = %q", got)
	}
}

func Test_Validate(t *testing.T) {
	for _, tt := range []struct {
		in      string
		wantErr bool
	}{
		{"NL07BUNQ2108214623", false},
		{"NL32BUNQ2025313705", false},
		{"DE89370400440532013000", false},
		{"GB82WEST12345698765432", false},
		{"NL08BUNQ2108214623", true}, // one digit off
		{"NL07BUNQ21082146", true},   // too short for the checksum
		{"07NLBUNQ2108214623", true}, // country code not letters
		{"NL07BUNQ2108-14623", true},
		{"", true},
	} {
		if err := Validate(tt.in); (err != nil) != tt.wantErr {
			t.Errorf("Validate(%q) = %v, wantErr %v", tt.in, err, tt.wantErr)
		}
	}
}
