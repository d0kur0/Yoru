package core

import (
	"math"
	"strconv"

	"gopkg.in/yaml.v3"
)

// marshalMihomoYAML restores integer scalars after JSON round-trips turn numbers
// in Server.Options into float64. Mihomo's XHTTP range fields accept decimal
// integers, but reject floats converted to strings such as "1E+06".
// Work on the encoded tree so nested maps/slices are covered without mutating
// saved options, converting strings, or truncating fractional values.
func marshalMihomoYAML(value any) ([]byte, error) {
	var node yaml.Node
	if err := node.Encode(value); err != nil {
		return nil, err
	}
	var normalize func(*yaml.Node)
	normalize = func(n *yaml.Node) {
		if n.Kind == yaml.ScalarNode && n.Tag == "!!float" {
			v, err := strconv.ParseFloat(n.Value, 64)
			if err == nil && !math.IsInf(v, 0) && !math.IsNaN(v) && math.Trunc(v) == v {
				n.Tag = "!!int"
				n.Value = strconv.FormatFloat(v, 'f', -1, 64)
			}
		}
		for _, child := range n.Content {
			normalize(child)
		}
	}
	normalize(&node)
	return yaml.Marshal(&node)
}
