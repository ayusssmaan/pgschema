package ir

import "testing"

// stripSameSchemaPrefix must strip a prefix matching either the routine's own
// schema or any schema known to host an installed extension (issue #518):
// when introspecting pgschema's own temporary comparison schema, a function's
// routineSchema *is* that temp schema, so an extension-owned type's real
// schema qualifier (e.g. "domain.vector") never matches routineSchema and
// would otherwise survive unstripped - causing the temp-schema side and the
// real-target side of a diff to compare as different function signatures.
func TestStripSameSchemaPrefix_ExtensionSchemaAware(t *testing.T) {
	tests := []struct {
		name             string
		typeName         string
		routineSchema    string
		extensionSchemas map[string]bool
		want             string
	}{
		{
			name:             "strips routine's own schema, no extension schemas known",
			typeName:         "domain.mytype",
			routineSchema:    "domain",
			extensionSchemas: nil,
			want:             "mytype",
		},
		{
			name:             "routine schema is the temp schema, but type is protected by an extension schema",
			typeName:         "domain.vector",
			routineSchema:    "pgschema_tmp_20260101_000000_abcd1234",
			extensionSchemas: map[string]bool{"domain": true},
			want:             "vector",
		},
		{
			name:             "quoted extension schema qualifier",
			typeName:         `"domain".vector`,
			routineSchema:    "pgschema_tmp_xxx",
			extensionSchemas: map[string]bool{"domain": true},
			want:             "vector",
		},
		{
			name:             "cross-schema type unaffected - schema matches neither routine nor any extension",
			typeName:         "utils.hstore",
			routineSchema:    "pgschema_tmp_xxx",
			extensionSchemas: map[string]bool{"domain": true},
			want:             "utils.hstore",
		},
		{
			name:             "already-bare type unaffected",
			typeName:         "vector",
			routineSchema:    "pgschema_tmp_xxx",
			extensionSchemas: map[string]bool{"domain": true},
			want:             "vector",
		},
		{
			name:             "empty type name",
			typeName:         "",
			routineSchema:    "domain",
			extensionSchemas: map[string]bool{"domain": true},
			want:             "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insp := &Inspector{extensionSchemas: tt.extensionSchemas}
			if got := insp.stripSameSchemaPrefix(tt.typeName, tt.routineSchema); got != tt.want {
				t.Errorf("stripSameSchemaPrefix(%q, %q) = %q, want %q", tt.typeName, tt.routineSchema, got, tt.want)
			}
		})
	}
}
