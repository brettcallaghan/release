package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func yamlOf(t *testing.T, n yamlNode) string {
	t.Helper()
	var out bytes.Buffer
	if err := writeYAML(&out, n); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestYAMLStyle(t *testing.T) {
	got := yamlOf(t, yMap{
		{"text", yStr("plain words")},
		{"version", yStr("1.10.0")},
		{"number_like", yStr("1.10")},
		{"bool_like", yStr("yes")},
		{"null_like", yStr("~")},
		{"empty", yStr("")},
		{"colon", yStr("feat: add")},
		{"hash", yStr("a #tag")},
		{"trailing_colon", yStr("key:")},
		{"indicator", yStr("-dash")},
		{"padded", yStr(" padded")},
		{"control", yStr("bell\a")},
		{"multi", yStr("\n  first\n\nthird  \n")},
		{"carriage", yStr("a\r\nb")},
		{"count", yInt(3)},
		{"flag", yBool(false)},
		{"absent", yNull()},
		{"empty_list", ySeq{}},
		{"empty_map", yMap{}},
		{"items", ySeq{yMap{{"a", yStr("x")}, {"b", yStr("w")}}, yStr("z")}},
		{"nested", yMap{{"inner", yStr("v")}}},
		{"odd:key", yStr("v")},
	})
	want := `text: plain words
version: 1.10.0
number_like: "1.10"
bool_like: "yes"
null_like: "~"
empty: ""
colon: "feat: add"
hash: "a #tag"
trailing_colon: "key:"
indicator: "-dash"
padded: " padded"
control: "bell\a"
multi: |-
  first

  third
carriage: "a\r\nb"
count: 3
flag: false
absent: null
empty_list: []
empty_map: {}
items:
  - a: x
    b: w
  - z
nested:
  inner: v
"odd:key": v
`
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestJSONKeepsTheOrderTheCommandReported(t *testing.T) {
	n := yMap{
		{"z", yStr("last letter, first key")},
		{"a", ySeq{yInt(1), yBool(true), yNull()}},
		{"m", yMap{}},
	}
	var out bytes.Buffer
	if err := writeJSON(&out, n); err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"z\": \"last letter, first key\",\n  \"a\": [\n    1,\n    true,\n    null\n  ],\n  \"m\": {}\n}\n"
	if out.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", out.String(), want)
	}
	if !json.Valid(out.Bytes()) {
		t.Error("output is not valid JSON")
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("closed") }

func TestWriteErrorsAreReturned(t *testing.T) {
	if err := writeYAML(failingWriter{}, yStr("x")); err == nil {
		t.Error("writeYAML: want the writer's error")
	}
	if err := writeJSON(failingWriter{}, yStr("x")); err == nil {
		t.Error("writeJSON: want the writer's error")
	}
}
