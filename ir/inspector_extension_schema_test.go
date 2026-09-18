package ir

import "testing"

// stripSameSchemaPrefix must strip a prefix matching either the routine's own
// schema, or (only when routineSchema is pgschema's own temp comparison
// schema, pgschema_tmp_*, AND the specific type is a catalog-verified
// extension member) a schema known to host an installed extension (issue
// #518): when introspecting that temp schema, a function's routineSchema *is*
// the temp schema name, so an extension-owned type's real schema qualifier
// (e.g. "domain.vector") never matches routineSchema and would otherwise
// survive unstripped - causing the temp-schema side and the real-target side
// of a diff to compare as different function signatures.
//
// Critically, this must NOT fire for a routine's genuine, permanent schema:
// a function actually declared in "app" taking a "domain.vector" parameter
// is a real cross-schema reference and must stay qualified, or generated
// CREATE/DROP/GRANT DDL would reference an unresolvable bare "vector" (a
// regression caught in PR #608 review - see the temp-schema-prefix guard).
//
// It also must NOT fire for a type that merely lives in a schema an
// extension happens to occupy but isn't itself an extension member (a schema
// can host both) - another regression caught in PR #608 review, addressed by
// checking extensionOwnedTypes rather than extensionSchemas alone.
func TestStripSameSchemaPrefix_ExtensionSchemaAware(t *testing.T) {
	tests := []struct {
		name                string
		typeName            string
		routineSchema       string
		extensionSchemas    map[string]bool
		extensionOwnedTypes map[string]bool
		want                string
	}{
		{
			name:             "strips routine's own schema, no extension schemas known",
			typeName:         "domain.mytype",
			routineSchema:    "domain",
			extensionSchemas: nil,
			want:             "mytype",
		},
		{
			name:                "routine schema is the temp schema, and type is a confirmed extension member",
			typeName:            "domain.vector",
			routineSchema:       "pgschema_tmp_20260101_000000_abcd1234",
			extensionSchemas:    map[string]bool{"domain": true},
			extensionOwnedTypes: map[string]bool{"domain.vector": true},
			want:                "vector",
		},
		{
			name:                "quoted extension schema qualifier",
			typeName:            `"domain".vector`,
			routineSchema:       "pgschema_tmp_xxx",
			extensionSchemas:    map[string]bool{"domain": true},
			extensionOwnedTypes: map[string]bool{"domain.vector": true},
			want:                "vector",
		},
		{
			name:                "array of a confirmed extension member type",
			typeName:            "domain.vector[]",
			routineSchema:       "pgschema_tmp_xxx",
			extensionSchemas:    map[string]bool{"domain": true},
			extensionOwnedTypes: map[string]bool{"domain.vector": true},
			want:                "vector[]",
		},
		{
			name:                "genuine cross-schema reference in a real (non-temp) schema is preserved",
			typeName:            "domain.vector",
			routineSchema:       "app",
			extensionSchemas:    map[string]bool{"domain": true},
			extensionOwnedTypes: map[string]bool{"domain.vector": true},
			want:                "domain.vector",
		},
		{
			name:                "schema hosts an extension, but this specific type is not a member",
			typeName:            "exts.status",
			routineSchema:       "pgschema_tmp_xxx",
			extensionSchemas:    map[string]bool{"exts": true},
			extensionOwnedTypes: map[string]bool{"exts.vector": true},
			want:                "exts.status",
		},
		{
			name:                "cross-schema type unaffected - schema matches neither routine nor any extension",
			typeName:            "utils.hstore",
			routineSchema:       "pgschema_tmp_xxx",
			extensionSchemas:    map[string]bool{"domain": true},
			extensionOwnedTypes: map[string]bool{"domain.vector": true},
			want:                "utils.hstore",
		},
		{
			name:                "already-bare type unaffected",
			typeName:            "vector",
			routineSchema:       "pgschema_tmp_xxx",
			extensionSchemas:    map[string]bool{"domain": true},
			extensionOwnedTypes: map[string]bool{"domain.vector": true},
			want:                "vector",
		},
		{
			name:                "empty type name",
			typeName:            "",
			routineSchema:       "domain",
			extensionSchemas:    map[string]bool{"domain": true},
			extensionOwnedTypes: map[string]bool{"domain.vector": true},
			want:                "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insp := &Inspector{extensionSchemas: tt.extensionSchemas, extensionOwnedTypes: tt.extensionOwnedTypes}
			if got := insp.stripSameSchemaPrefix(tt.typeName, tt.routineSchema); got != tt.want {
				t.Errorf("stripSameSchemaPrefix(%q, %q) = %q, want %q", tt.typeName, tt.routineSchema, got, tt.want)
			}
		})
	}
}

// stripSameSchemaPrefixFromReturnType must decompose SETOF and TABLE(...)
// return types the same way ir/normalize.go's stripSchemaFromReturnType
// does, applying the extension-membership-aware stripSameSchemaPrefix to
// each contained type rather than a single top-level prefix check. Without
// this, "RETURNS vector" would compare as "domain.vector" (temp side) vs
// "vector" (real target side) and spuriously trigger a drop+recreate (PR
// #608 review feedback).
func TestStripSameSchemaPrefixFromReturnType(t *testing.T) {
	insp := &Inspector{
		extensionSchemas:    map[string]bool{"domain": true},
		extensionOwnedTypes: map[string]bool{"domain.vector": true},
	}

	tests := []struct {
		name          string
		returnType    string
		routineSchema string
		want          string
	}{
		{
			name:          "direct extension-owned return type in temp schema",
			returnType:    "domain.vector",
			routineSchema: "pgschema_tmp_xxx",
			want:          "vector",
		},
		{
			name:          "SETOF extension-owned return type in temp schema",
			returnType:    "SETOF domain.vector",
			routineSchema: "pgschema_tmp_xxx",
			want:          "SETOF vector",
		},
		{
			name:          "TABLE(...) column referencing an extension-owned type in temp schema",
			returnType:    "TABLE(id integer, embedding domain.vector)",
			routineSchema: "pgschema_tmp_xxx",
			want:          "TABLE(id integer, embedding vector)",
		},
		{
			name:          "direct extension-owned return type on a real, permanent schema is preserved",
			returnType:    "domain.vector",
			routineSchema: "app",
			want:          "domain.vector",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := insp.stripSameSchemaPrefixFromReturnType(tt.returnType, tt.routineSchema); got != tt.want {
				t.Errorf("stripSameSchemaPrefixFromReturnType(%q, %q) = %q, want %q", tt.returnType, tt.routineSchema, got, tt.want)
			}
		})
	}
}

// stripExtensionMemberTypeQualifiers is buildPrivileges' equivalent of
// stripSameSchemaPrefix's extension-membership check, operating on a whole
// function/procedure identity-arguments string instead of a single type.
func TestStripExtensionMemberTypeQualifiers(t *testing.T) {
	insp := &Inspector{
		extensionSchemas:    map[string]bool{"domain": true},
		extensionOwnedTypes: map[string]bool{"domain.vector": true},
	}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "strips a confirmed extension member type in a signature",
			in:   "vector_search(query_embedding domain.vector)",
			want: "vector_search(query_embedding vector)",
		},
		{
			name: "preserves a non-member type in the same extension schema",
			in:   "f(x domain.status)",
			want: "f(x domain.status)",
		},
		{
			name: "leaves an unrelated schema untouched",
			in:   "g(x utils.hstore)",
			want: "g(x utils.hstore)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := insp.stripExtensionMemberTypeQualifiers(tt.in); got != tt.want {
				t.Errorf("stripExtensionMemberTypeQualifiers(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
