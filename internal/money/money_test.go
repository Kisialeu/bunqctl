package money

import "testing"

func Test_Parse(t *testing.T) {
	tests := []struct {
		in      string
		want    Cents
		wantErr bool
	}{
		{"12.34", 1234, false},
		{"0.01", 1, false},
		{"500.00", 50000, false},
		{"0.00", 0, true},
		{"12.3", 0, true},
		{"12", 0, true},
		{"-1.00", 0, true},
		{"1,00", 0, true},
		{"1e3.00", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := Parse(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Parse(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("Parse(%q) = %d, want %d", tt.in, got, tt.want)
			}
			if err == nil && got.String() != tt.in {
				t.Errorf("String() = %q, want %q", got.String(), tt.in)
			}
		})
	}
}

func Test_ParseAPI(t *testing.T) {
	tests := []struct {
		in      string
		want    Cents
		wantErr bool
	}{
		{"-12.34", -1234, false},
		{"-0.05", -5, false},
		{"0.00", 0, false},
		{"500.00", 50000, false},
		{"--1.00", 0, true},
		{"1.5", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseAPI(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseAPI(%q) error = %v, wantErr %v", tt.in, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("ParseAPI(%q) = %d, want %d", tt.in, got, tt.want)
			}
			if err == nil && got.String() != tt.in {
				t.Errorf("String() = %q, want %q", got.String(), tt.in)
			}
		})
	}
}
