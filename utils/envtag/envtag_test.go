package envtag

import (
	"os"
	"reflect"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	type OtherStruct struct {
		FieldD string `env:"FIELD_D"`
	}
	type TestStruct struct {
		FieldA      string `env:"FIELD_A"`
		FieldB      string `env:"field_b"`
		FieldC      string
		OtherStruct `env:",squash"`
	}
	want := &TestStruct{
		FieldA:      "abcdef",
		FieldB:      "ghi123",
		FieldC:      "456",
		OtherStruct: OtherStruct{FieldD: "field d"},
	}

	os.Setenv("PREFIX_FIELD_A", want.FieldA)
	os.Setenv("PREFIX_FIELD_B", want.FieldB)
	os.Setenv("PREFIX_FIELD_D", want.FieldD)

	s := &TestStruct{FieldC: "456"}
	Unmarshal("env", "PREFIX_", s)

	if !reflect.DeepEqual(s, want) {
		t.Errorf("got %#v, want %#v", s, want)
	}
}
