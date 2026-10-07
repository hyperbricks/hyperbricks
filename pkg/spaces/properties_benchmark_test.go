package spaces

import (
	"fmt"
	"testing"
)

// These fixtures use the pre-extension contract so results can be compared with
// the parent revision. Asset I/O and native image processing are excluded.
func BenchmarkSpacesDiscovery(b *testing.B) {
	for _, tc := range []struct {
		name                         string
		depth, width, fields, spaces int
	}{
		{"deep", 40, 1, 10, 1}, {"wide", 1, 100, 10, 1}, {"many-fields", 1, 1, 1000, 1}, {"shared-source-50-spaces", 4, 1, 20, 50},
	} {
		b.Run(tc.name, func(b *testing.B) {
			root := map[string]interface{}{"@type": "<HYPERMEDIA>"}
			for w := 0; w < tc.width; w++ {
				node := map[string]interface{}{}
				root[fmt.Sprintf("branch%d", w)] = node
				for d := 0; d < tc.depth; d++ {
					child := map[string]interface{}{}
					node[fmt.Sprintf("nested%d", d)] = child
					node = child
				}
				values, editable := map[string]interface{}{}, map[string]interface{}{}
				for f := 0; f < tc.fields; f++ {
					key := fmt.Sprintf("field%d", f)
					values[key] = "Content"
					editable[key] = "text"
				}
				node[fmt.Sprintf("component%d", w)] = map[string]interface{}{"@type": "<TEMPLATE>", "values": values, "editable": editable}
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				for s := 0; s < tc.spaces; s++ {
					fields, err := SourceFields(root)
					if err != nil || len(fields) != tc.width*tc.fields {
						b.Fatalf("fields=%d err=%v", len(fields), err)
					}
				}
			}
		})
	}
}
