package apiutil

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"
)

func TestMapRequestBodyJSON(t *testing.T) {
	tests := []struct {
		name, body, want string
		input            map[string]interface{}
	}{
		{"missing only member", `{"id":"$id"}`, `{}`, nil},
		{"missing first member", `{"id":"$id","fixed":7}`, `{"fixed":7}`, nil},
		{"missing middle member", `{"a":1,"id":"$id","b":2}`, `{"a":1,"b":2}`, nil},
		{"missing last member", `{"fixed":7,"id":"$id"}`, `{"fixed":7}`, nil},
		{"missing bare member", `{"id":$id}`, `{}`, nil},
		{"missing nested member", `{"user":{"id":"$id"},"rows":[{"id":$id,"fixed":7}]}`, `{"user":{},"rows":[{"fixed":7}]}`, nil},
		{"empty string is supplied", `{"id":"$id"}`, `{"id":""}`, map[string]interface{}{"id": ""}},
		{"null in quoted slot", `{"id":"$id"}`, `{"id":null}`, map[string]interface{}{"id": nil}},
		{"null in bare slot", `{"id":$id}`, `{"id":null}`, map[string]interface{}{"id": nil}},
		{"quoted values stay strings", `{"count":"$count","enabled":"$enabled"}`, `{"count":"0","enabled":"false"}`, map[string]interface{}{"count": 0, "enabled": false}},
		{"bare values retain types", `{"count":$count,"enabled":$enabled}`, `{"count":0,"enabled":false}`, map[string]interface{}{"count": 0, "enabled": false}},
		{"bare URL string is not a number", `{"id":$id}`, `{"id":"7"}`, map[string]interface{}{"id": "7"}},
		{"bare array and object", `{"ids":$ids,"user":$user}`, `{"ids":["7","8"],"user":{"id":7}}`, map[string]interface{}{"ids": []string{"7", "8"}, "user": map[string]interface{}{"id": 7}}},
		{"do not map supplied text again", `{"id":"$id","other":$other}`, `{"id":"$missing","other":{"value":"$id"}}`, map[string]interface{}{"id": "$missing", "other": map[string]interface{}{"value": "$id"}}},
		{"literal property names", `{"$id":"fixed","id":"$id"}`, `{"$id":"fixed","id":"7"}`, map[string]interface{}{"id": "7"}},
		{"escaped input is data", `{"id":"$id"}`, `{"id":"\"},\"admin\":true,\"x\":\"\\\n"}`, map[string]interface{}{"id": "\"},\"admin\":true,\"x\":\"\\\n"}},
		{"interpolated supplied input", `{"label":"item-$id"}`, `{"label":"item-7"}`, map[string]interface{}{"id": "7"}},
		{"offsets with UTF8 and whitespace", "{\n\"één\": \"fixed\", \"twee\": $twee, \"drie\": $drie }", `{"één":"fixed","twee":false}`, map[string]interface{}{"twee": false}},
		{"escaped template string and bare slot", `{"label":"a\\\"$id","n":$n}`, `{"label":"a\\\"7","n":5}`, map[string]interface{}{"id": "7", "n": 5}},
		{"literal number precision", `{"constant":9007199254740993,"id":"$id"}`, `{"constant":9007199254740993}`, nil},
		{"top level array members", `[{"id":"$id"},$other]`, `[{},null]`, map[string]interface{}{"other": nil}},
		{"top level quoted value", `"$id"`, `"7"`, map[string]interface{}{"id": "7"}},
		{"top level bare value", `$id`, `7`, map[string]interface{}{"id": 7}},
		{"whole escaped placeholder", `{"id":"\u0024id"}`, `{}`, nil},
		{"escaped placeholder beside supplied value", `{"id":"\u0024id","other":"$other"}`, `{"other":"7"}`, map[string]interface{}{"other": "7"}},
		{"escaped placeholder name", `{"id":"$\u0069d"}`, `{"id":"7"}`, map[string]interface{}{"id": "7"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := MapRequestBody(test.body, test.input)
			if err != nil {
				t.Fatal(err)
			}
			decode := func(text string) interface{} {
				decoder := json.NewDecoder(strings.NewReader(text))
				decoder.UseNumber()
				var value interface{}
				if err := decoder.Decode(&value); err != nil {
					t.Fatalf("invalid JSON %q: %v", text, err)
				}
				return value
			}
			if !reflect.DeepEqual(decode(got), decode(test.want)) {
				t.Fatalf("body=%s, want %s", got, test.want)
			}
		})
	}
}

func TestMapRequestBodyRejectsAmbiguousOrInvalidJSON(t *testing.T) {
	for _, body := range []string{
		`{"label":"item-$id"}`, `{"label":"item-$null"}`,
		`["$id"]`, `[$id]`, `"$id"`, `$id`,
		`{"id":$id,}`, `{"id":$id} {}`, `{"id":$id+1}`,
		`{"id":$bad}`, `{"id":$nan}`,
	} {
		t.Run(body, func(t *testing.T) {
			got, err := MapRequestBody(body, map[string]interface{}{"null": nil, "bad": make(chan int), "nan": math.NaN()})
			if err == nil || got != "" {
				t.Fatalf("body=%q error=%v; want no body and a preparation error", got, err)
			}
			if strings.Contains(err.Error(), body) {
				t.Fatalf("error discloses configured body: %v", err)
			}
		})
	}
}

func TestMapRequestBodyPreservesLiteralAndRawBodies(t *testing.T) {
	for _, test := range []struct{ body, want string }{
		{"", ""},
		{"{\n  \"fixed\": true\n}", "{\n  \"fixed\": true\n}"},
		{"raw $id $missing", "raw 7 $missing"},
		{"<id>$id</id>", "<id>7</id>"},
	} {
		got, err := MapRequestBody(test.body, map[string]interface{}{"id": "7"})
		if err != nil || got != test.want {
			t.Fatalf("MapRequestBody(%q)=(%q, %v), want %q", test.body, got, err, test.want)
		}
	}
}
