package expr

import (
	"strings"
	"testing"
)

func TestGoodParse(t *testing.T) {
	tests := []struct {
		src     string
		postfix string
	}{
		{"a.b.c == 2", "EQ($a.b.c, 2)"},
		{`!(a.b || b || "c a b" || ( a && b ))`, `NOT(LOR(LOR(LOR($a.b, $b), "c a b"), LAND($a, $b)))`},
		{`a || b || "c" || ( a && b )`, `LOR(LOR(LOR($a, $b), "c"), LAND($a, $b))`},
		{`substring(a, 5) == "abc"`, `EQ(substring($a, 5), "abc")`},
		{`a|b|c`, `OR(OR($a, $b), $c)`},
		{`200`, `200`},
		{`origin.host == "9.0.10.1"`, `EQ($origin.host, "9.0.10.1")`},
		{`service.name == "cluster1.ns.*"`, `EQ($service.name, "cluster1.ns.*")`},
		{`a() == 200`, `EQ(a(), 200)`},
		{`true == false`, `EQ(true, false) `},
	}
	for _, tt := range tests {
		ex, err := Parse(tt.src)
		if err != nil {
			t.Error(err)
			continue
		}
		if tt.postfix != ex.String() {
			t.Errorf("got %s\nwant: %s", ex.String(), tt.postfix)
		}
	}
}

func TestBadParse(t *testing.T) {
	tests := []struct {
		src string
		err string
	}{
		{`*a != b`, "unexpected expression"},
		{"a = bc", `parse error`},
		{`3 = 10`, "parse error"},
		{`a.b.c() == 20`, "unexpected expression"},
		{`(a.c).d == 300`, `unexpected expression`},
	}
	for _, tt := range tests {
		_, err := Parse(tt.src)
		if err == nil {
			t.Errorf("got: <nil>\nwant: %s", tt.err)
			continue
		}

		if !strings.Contains(err.Error(), tt.err) {
			t.Errorf("got: %s\nwant: %s", err.Error(), tt.err)
		}
	}

	// nil test
	tex := &Expression{}
	if tex.String() != "<nil>" {
		t.Errorf("got: %s\nwant: <nil>", tex.String())
	}
}
