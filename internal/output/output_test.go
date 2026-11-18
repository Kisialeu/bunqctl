package output

import (
	"bytes"
	"encoding/json"
	"testing"
)

type row struct {
	Name   string
	Amount string
}

var cols = []Column[row]{
	{Header: "NAME", Value: func(r row) string { return r.Name }},
	{Header: "AMOUNT", Value: func(r row) string { return r.Amount }, Right: true},
}

func view(r row) map[string]string { return map[string]string{"name": r.Name} }

func Test_List_table(t *testing.T) {
	var b bytes.Buffer
	rows := []row{{"Coffee", "-7.50"}, {"Salary", "1000.00"}}
	if err := List(Printer{W: &b}, rows, view, cols); err != nil {
		t.Fatal(err)
	}
	want := "NAME     AMOUNT\n" +
		"Coffee    -7.50\n" +
		"Salary  1000.00\n"
	if b.String() != want {
		t.Errorf("table:\n%s\nwant:\n%s", b.String(), want)
	}
}

func Test_List_jsonUsesView(t *testing.T) {
	var b bytes.Buffer
	if err := List(Printer{W: &b, JSON: true}, []row{{"Coffee", "-7.50"}}, view, cols); err != nil {
		t.Fatal(err)
	}
	var got []map[string]string
	if err := json.Unmarshal(b.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0]["name"] != "Coffee" || len(got[0]) != 1 {
		t.Errorf("json = %v", got)
	}
}

func Test_List_emptyJSONIsArray(t *testing.T) {
	var b bytes.Buffer
	if err := List(Printer{W: &b, JSON: true}, []row(nil), view, cols); err != nil {
		t.Fatal(err)
	}
	if b.String() != "[]\n" {
		t.Errorf("json = %q, want []", b.String())
	}
}

func Test_Fields_table(t *testing.T) {
	var b bytes.Buffer
	if err := (Printer{W: &b}).Fields(nil, [][2]string{{"ID", "1"}, {"Name", "Test"}}); err != nil {
		t.Fatal(err)
	}
	if want := "ID:    1\nName:  Test\n"; b.String() != want {
		t.Errorf("fields = %q, want %q", b.String(), want)
	}
}
