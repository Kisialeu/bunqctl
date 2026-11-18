package profile

import (
	"errors"
	"os"
	"testing"
	"time"
)

func Test_ParseEnv(t *testing.T) {
	for _, tt := range []struct {
		in      string
		want    Env
		wantErr bool
	}{
		{"sandbox", Sandbox, false},
		{"prod", Production, false},
		{"production", "", true},
		{"", "", true},
	} {
		got, err := ParseEnv(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseEnv(%q) = %q, %v", tt.in, got, err)
		}
	}
}

func Test_ValidateIPs(t *testing.T) {
	for _, tt := range []struct {
		name    string
		ips     []string
		wantErr bool
	}{
		{"one", []string{"203.0.113.1"}, false},
		{"several", []string{"203.0.113.1", "198.51.100.7"}, false},
		{"empty", nil, true},
		{"ipv6", []string{"2001:db8::1"}, true},
		{"garbage", []string{"vpn"}, true},
		{"wildcard", []string{"*"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateIPs(tt.ips); (err != nil) != tt.wantErr {
				t.Errorf("ValidateIPs(%v) = %v, wantErr %v", tt.ips, err, tt.wantErr)
			}
		})
	}
}

func Test_SaveLoad_roundTrip(t *testing.T) {
	dir := t.TempDir()
	want := &Profile{
		Env: Sandbox, PermittedIPs: []string{"203.0.113.1"}, Country: "DE",
		ServerPublicKey: "pem", InstallationID: 1, DeviceID: 2, UserID: 3,
		SessionCreated: time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC),
	}
	if err := want.Save(dir); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir, Sandbox)
	if err != nil {
		t.Fatal(err)
	}
	if got.UserID != 3 || got.DeviceID != 2 || got.Country != "DE" || !got.SessionCreated.Equal(want.SessionCreated) {
		t.Errorf("Load = %+v", got)
	}
	if !got.Initialized() {
		t.Error("Initialized() = false")
	}
	if got.Geolocation() != "0 0 0 0 DE" {
		t.Errorf("Geolocation() = %q", got.Geolocation())
	}

	info, err := os.Stat(Path(dir, Sandbox))
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("mode = %o, want 600", mode)
	}
}

func Test_Load_missing(t *testing.T) {
	if _, err := Load(t.TempDir(), Production); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func Test_Delete_missingIsNotAnError(t *testing.T) {
	if err := Delete(t.TempDir(), Sandbox); err != nil {
		t.Error(err)
	}
}

func Test_Geolocation_default(t *testing.T) {
	if got := (&Profile{}).Geolocation(); got != "0 0 0 0 NL" {
		t.Errorf("Geolocation() = %q", got)
	}
}
