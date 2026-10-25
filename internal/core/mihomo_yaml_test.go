package core

import (
	"math"
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestMihomoYAMLNumberTypes(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		tag   string
		text  string
	}{
		{"integer", 1000000, "!!int", "1000000"},
		{"JSON integer", float64(1000000), "!!int", "1000000"},
		{"float32 integer", float32(1000000), "!!int", "1000000"},
		{"negative", float64(-1000000), "!!int", "-1000000"},
		{"zero", float64(0), "!!int", "0"},
		{"large integer", float64(1e18), "!!int", "1000000000000000000"},
		{"uint64", uint64(math.MaxUint64), "!!int", "18446744073709551615"},
		{"fraction", 1.25, "!!float", "1.25"},
		{"small fraction", 1e-9, "!!float", "1e-09"},
		{"numeric string", "1000000", "!!str", "1000000"},
		{"exponent string", "1e+06", "!!str", "1e+06"},
		{"range", "100-1000", "!!str", "100-1000"},
		{"boolean", true, "!!bool", "true"},
		{"null", nil, "!!null", "null"},
	} {
		t.Run(test.name, func(t *testing.T) {
			input := map[string]any{"nested": []any{map[string]any{"value": test.value}}}
			data, err := marshalMihomoYAML(input)
			if err != nil {
				t.Fatal(err)
			}
			var node yaml.Node
			if err = yaml.Unmarshal(data, &node); err != nil {
				t.Fatal(err)
			}
			value := node.Content[0].Content[1].Content[0].Content[1]
			if value.Tag != test.tag || value.Value != test.text {
				t.Fatalf("got %s %q, want %s %q", value.Tag, value.Value, test.tag, test.text)
			}
			if !reflect.DeepEqual(input["nested"].([]any)[0].(map[string]any)["value"], test.value) {
				t.Fatal("source options mutated")
			}
		})
	}
}
