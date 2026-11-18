package bunq

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func Test_CancelDraftPayment_rejectsWithTimestamp(t *testing.T) {
	var put map[string]any
	srv := fakeBunq(t, func(r *http.Request, body []byte) (int, string) {
		if r.Method == http.MethodPut {
			if err := json.Unmarshal(body, &put); err != nil {
				t.Error(err)
			}
			return 200, `{"Response":[{"Id":{"id":5}}]}`
		}
		return 200, `{"Response":[{"DraftPayment":{"id":5,"status":"PENDING","updated":"2026-11-18 11:00:00.000001"}}]}`
	})
	if err := newTestClient(srv, nil).CancelDraftPayment(context.Background(), 1, 2, 5); err != nil {
		t.Fatal(err)
	}
	if put["status"] != "REJECTED" || put["previous_updated_timestamp"] != "2026-11-18 11:00:00.000001" || len(put) != 2 {
		t.Errorf("PUT body = %v", put)
	}
}

func Test_SetCardStatus_refusesPermanentStatuses(t *testing.T) {
	srv := fakeBunq(t, func(*http.Request, []byte) (int, string) {
		t.Error("request sent for a refused status")
		return 200, `{}`
	})
	for _, status := range []string{"LOST", "STOLEN", "CANCELLED", "active"} {
		if err := newTestClient(srv, nil).SetCardStatus(context.Background(), 1, 2, status); err == nil {
			t.Errorf("status %s was accepted", status)
		}
	}
}

func Test_UploadAttachment_sendsSignedRawBody(t *testing.T) {
	data := []byte("\x89PNG fake image")
	srv := fakeBunq(t, func(r *http.Request, body []byte) (int, string) {
		if string(body) != string(data) {
			t.Errorf("body = %q", body)
		}
		if ct := r.Header.Get("Content-Type"); ct != "image/png" {
			t.Errorf("Content-Type = %q", ct)
		}
		if d := r.Header.Get("X-Bunq-Attachment-Description"); d != "receipt" {
			t.Errorf("description header = %q", d)
		}
		return 200, `{"Response":[{"Id":{"id":77}}]}`
	})
	id, err := newTestClient(srv, nil).UploadAttachment(context.Background(), 1, 2, "image/png", "receipt", data)
	if err != nil || id != 77 {
		t.Fatalf("id=%d err=%v", id, err)
	}
}

func Test_StatementContent_returnsVerifiedRawBody(t *testing.T) {
	srv := fakeBunq(t, func(*http.Request, []byte) (int, string) {
		return 200, "\"Date\";\"Amount\"\n"
	})
	got, err := newTestClient(srv, nil).StatementContent(context.Background(), 1, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "\"Date\";\"Amount\"\n" {
		t.Errorf("content = %q", got)
	}
}

func Test_Insights_sendsPeriod(t *testing.T) {
	srv := fakeBunq(t, func(r *http.Request, _ []byte) (int, string) {
		q := r.URL.Query()
		if q.Get("time_start") != "2026-11-01 00:00:00" || q.Get("time_end") != "2026-11-30 00:00:00" {
			t.Errorf("query = %v", q)
		}
		return 200, `{"Response":[{"InsightCategory":{"category":{"category":"GENERAL","description_translated":"General"},
			"amount_total":{"value":"32.18","currency":"EUR"},"number_of_transactions":3}}]}`
	})
	from := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	got, err := collectAll(newTestClient(srv, nil).Insights(context.Background(), 1, from, from.AddDate(0, 0, 29)))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Category.Description != "General" || got[0].Total.Value != "32.18" || got[0].Transactions != 3 {
		t.Errorf("insights = %+v", got)
	}
}

func Test_EventObject_decodesAnyType(t *testing.T) {
	for _, tt := range []struct {
		in, typ, amount string
	}{
		{`{"Payment":{"amount":{"value":"-7.50"},"description":"Coffee","counterparty_alias":{"display_name":"Cafe"}}}`, "Payment", "-7.50"},
		{`{"MasterCardAction":{"amount_billing":{"value":"-3.00"},"description":"Card"}}`, "MasterCardAction", "-3.00"},
		{`{"RequestInquiry":{"amount_inquired":{"value":"500.00"}}}`, "RequestInquiry", "500.00"},
		{`{"ShareInvite":{}}`, "ShareInvite", ""},
	} {
		var o EventObject
		if err := json.Unmarshal([]byte(tt.in), &o); err != nil {
			t.Fatalf("%s: %v", tt.in, err)
		}
		got := ""
		if o.Amount != nil {
			got = o.Amount.Value
		}
		if o.Type != tt.typ || got != tt.amount {
			t.Errorf("%s: type=%s amount=%q", tt.in, o.Type, got)
		}
	}
}

func Test_Int_numberOrString(t *testing.T) {
	var v struct{ A, B, C Int }
	if err := json.Unmarshal([]byte(`{"A":12,"B":"34","C":null}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.A != 12 || v.B != 34 || v.C != 0 {
		t.Errorf("got %+v", v)
	}
}

func Test_ParseTime(t *testing.T) {
	want := time.Date(2026, 11, 30, 23, 0, 0, 0, time.UTC)
	for _, s := range []string{"2026-11-30 23:00:00.000000", "2026-11-30 23:00:00", "2026-11-30T23:00:00Z"} {
		got, err := ParseTime(s)
		if err != nil || !got.Equal(want) {
			t.Errorf("ParseTime(%q) = %v, %v", s, got, err)
		}
	}
	if _, err := ParseTime("30.11.2026"); err == nil {
		t.Error("accepted an unknown format")
	}
}
