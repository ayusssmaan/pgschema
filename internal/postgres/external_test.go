package postgres

import "testing"

// buildDesiredStateSearchPath must order the managed schema before "public"
// (not after), so its priority matches the real apply session's
// "<schema>, public" search_path (see cmd/apply/apply.go). Putting it after
// public would let a same-named object in public shadow the managed
// schema's extension type during planning while the real target resolves
// it the other way around (PR #608 review feedback).
func TestBuildDesiredStateSearchPath(t *testing.T) {
	tests := []struct {
		name             string
		tempSchema       string
		schema           string
		extensionSchemas map[string]string
		want             string
	}{
		{
			name:             "managed schema hosts an extension - inserted before public",
			tempSchema:       "pgschema_tmp_xxx",
			schema:           "domain",
			extensionSchemas: map[string]string{"vector": "domain"},
			want:             `"pgschema_tmp_xxx", "domain", public`,
		},
		{
			name:             "managed schema does not host an extension",
			tempSchema:       "pgschema_tmp_xxx",
			schema:           "domain",
			extensionSchemas: map[string]string{"vector": "other_schema"},
			want:             `"pgschema_tmp_xxx", public`,
		},
		{
			name:             "no extensions installed at all",
			tempSchema:       "pgschema_tmp_xxx",
			schema:           "domain",
			extensionSchemas: map[string]string{},
			want:             `"pgschema_tmp_xxx", public`,
		},
		{
			name:             "managed schema is public - never duplicated",
			tempSchema:       "pgschema_tmp_xxx",
			schema:           "public",
			extensionSchemas: map[string]string{"vector": "public"},
			want:             `"pgschema_tmp_xxx", public`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := buildDesiredStateSearchPath(tt.tempSchema, tt.schema, tt.extensionSchemas); got != tt.want {
				t.Errorf("buildDesiredStateSearchPath(%q, %q, %v) = %q, want %q", tt.tempSchema, tt.schema, tt.extensionSchemas, got, tt.want)
			}
		})
	}
}
