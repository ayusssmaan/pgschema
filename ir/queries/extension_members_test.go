package queries_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/pgplex/pgschema/ir/queries"
	"github.com/pgplex/pgschema/testutil"
	"github.com/stretchr/testify/require"
)

// Extension membership belongs to a catalog object, not a name, schema or
// dependency on an extension type. Use real ALTER EXTENSION ADD edges so these
// regressions run with bundled PostgreSQL on every supported major version.
func TestExtensionMembers(t *testing.T) {
	db, _, _, _, _, _ := testutil.ConnectToPostgres(t, sharedTestPostgres)
	defer db.Close()
	ctx := context.Background()
	const schemaName = "extension595"
	_, err := db.ExecContext(ctx, `CREATE SCHEMA "extension595";
 CREATE EXTENSION hstore SCHEMA "extension595";`)
	require.NoError(t, err)
	defer db.ExecContext(ctx, `DROP EXTENSION hstore CASCADE; DROP SCHEMA IF EXISTS "extension595" CASCADE`)

	for _, prefix := range []string{"member595", "app595"} {
		_, err = db.ExecContext(ctx, fmt.Sprintf(`
 SET search_path TO "extension595", pg_catalog;
 CREATE TABLE %[1]s_table (id serial PRIMARY KEY, attrs hstore, checked integer CHECK (checked > 0));
 CREATE INDEX %[1]s_index ON %[1]s_table (checked);
 CREATE VIEW %[1]s_view AS SELECT id FROM %[1]s_table;
 CREATE MATERIALIZED VIEW %[1]s_matview AS SELECT id FROM %[1]s_table;
 CREATE SEQUENCE %[1]s_sequence;
 CREATE TYPE %[1]s_enum AS ENUM ('one', 'two');
 CREATE TYPE %[1]s_composite AS (id integer);
 CREATE DOMAIN %[1]s_domain AS integer CHECK (VALUE > 0);
 CREATE FUNCTION %[1]s_function() RETURNS integer LANGUAGE sql AS 'SELECT 1';
 CREATE PROCEDURE %[1]s_procedure() LANGUAGE sql AS 'SELECT 1';
 CREATE AGGREGATE %[1]s_aggregate(integer) (SFUNC=int4pl, STYPE=integer, INITCOND='0');
 CREATE FUNCTION %[1]s_trigger_function() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NEW; END';
 CREATE TRIGGER %[1]s_trigger BEFORE INSERT ON %[1]s_table FOR EACH ROW EXECUTE FUNCTION %[1]s_trigger_function();
 ALTER TABLE %[1]s_table ENABLE ROW LEVEL SECURITY;
 CREATE POLICY %[1]s_policy ON %[1]s_table USING (id > 0);
 CREATE TABLE %[1]s_partitioned (id integer) PARTITION BY RANGE (id);
 CREATE TABLE %[1]s_partition PARTITION OF %[1]s_partitioned FOR VALUES FROM (0) TO (10);
 GRANT SELECT ON %[1]s_table, %[1]s_view, %[1]s_matview TO PUBLIC;
 GRANT SELECT (checked) ON %[1]s_table TO PUBLIC;
 GRANT USAGE ON SEQUENCE %[1]s_sequence, %[1]s_table_id_seq TO PUBLIC;
 REVOKE USAGE ON TYPE %[1]s_table FROM PUBLIC;
 REVOKE EXECUTE ON FUNCTION %[1]s_function() FROM PUBLIC;
 REVOKE EXECUTE ON PROCEDURE %[1]s_procedure() FROM PUBLIC;
 REVOKE USAGE ON TYPE %[1]s_enum FROM PUBLIC;
 `, prefix))
		require.NoError(t, err)
	}
	// Include a quoted member name and identically named application object in
	// another schema: object names cannot be the membership identity.
	_, err = db.ExecContext(ctx, `
 SET search_path TO "extension595", pg_catalog;
 CREATE TABLE "member595.Quoted" (id integer);
 ALTER EXTENSION hstore ADD TABLE "member595.Quoted";
 ALTER EXTENSION hstore ADD TABLE member595_table;

 ALTER EXTENSION hstore ADD VIEW member595_view;
 ALTER EXTENSION hstore ADD MATERIALIZED VIEW member595_matview;
 ALTER EXTENSION hstore ADD SEQUENCE member595_sequence;
 ALTER EXTENSION hstore ADD TYPE member595_enum;
 ALTER EXTENSION hstore ADD TYPE member595_composite;
 ALTER EXTENSION hstore ADD DOMAIN member595_domain;
 ALTER EXTENSION hstore ADD FUNCTION member595_function();
 ALTER EXTENSION hstore ADD PROCEDURE member595_procedure();
 ALTER EXTENSION hstore ADD AGGREGATE member595_aggregate(integer);
 ALTER EXTENSION hstore ADD FUNCTION member595_trigger_function();
 ALTER EXTENSION hstore ADD TABLE member595_partitioned;
 ALTER EXTENSION hstore ADD TABLE member595_partition;
 -- An independently created partition is an application table, even if its
 -- parent is an extension member. It still needs its PARTITION OF metadata.
 CREATE TABLE app595_external_partition PARTITION OF member595_partitioned
   FOR VALUES FROM (10) TO (20);
 ALTER FUNCTION app595_function() DEPENDS ON EXTENSION hstore;
 CREATE SCHEMA extension595_other;
 CREATE TABLE extension595_other.member595_table (id integer);
 ALTER EXTENSION hstore ADD SCHEMA extension595_other;
 `)
	require.NoError(t, err)
	defer db.ExecContext(ctx, `DROP SCHEMA IF EXISTS extension595_other CASCADE`)
	q := queries.New(db)
	schema := sql.NullString{String: schemaName, Valid: true}
	// Each getter must keep application objects and omit extension definitions,
	// children and ACLs, including explicitly changed member ACLs.
	tests := []struct {
		name string
		run  func() (string, error)
	}{
		{"GetTables", func() (string, error) { return extensionRows(q.GetTables(ctx)) }},
		{"GetTablesForSchema", func() (string, error) { return extensionRows(q.GetTablesForSchema(ctx, schema)) }},
		{"GetColumns", func() (string, error) { return extensionRows(q.GetColumns(ctx)) }},
		{"GetColumnsForSchema", func() (string, error) { return extensionRows(q.GetColumnsForSchema(ctx, schema)) }},
		{"GetConstraints", func() (string, error) { return extensionRows(q.GetConstraints(ctx)) }},
		{"GetConstraintsForSchema", func() (string, error) { return extensionRows(q.GetConstraintsForSchema(ctx, schema)) }},
		{"GetIndexes", func() (string, error) { return extensionRows(q.GetIndexes(ctx)) }},
		{"GetIndexesForSchema", func() (string, error) { return extensionRows(q.GetIndexesForSchema(ctx, schema)) }},
		{"GetSequences", func() (string, error) { return extensionRows(q.GetSequences(ctx)) }},
		{"GetSequencesForSchema", func() (string, error) { return extensionRows(q.GetSequencesForSchema(ctx, schema)) }},
		{"GetFunctions", func() (string, error) { return extensionRows(q.GetFunctions(ctx)) }},
		{"GetFunctionsForSchema", func() (string, error) { return extensionRows(q.GetFunctionsForSchema(ctx, schema)) }},
		{"GetProcedures", func() (string, error) { return extensionRows(q.GetProcedures(ctx)) }},
		{"GetProceduresForSchema", func() (string, error) { return extensionRows(q.GetProceduresForSchema(ctx, schema)) }},
		{"GetAggregates", func() (string, error) { return extensionRows(q.GetAggregates(ctx)) }},
		{"GetAggregatesForSchema", func() (string, error) { return extensionRows(q.GetAggregatesForSchema(ctx, schema)) }},
		{"GetViews", func() (string, error) { return extensionRows(q.GetViews(ctx)) }},
		{"GetViewsForSchema", func() (string, error) { return extensionRows(q.GetViewsForSchema(ctx, schema)) }},
		{"GetTypes", func() (string, error) { return extensionRows(q.GetTypes(ctx)) }},
		{"GetTypesForSchema", func() (string, error) { return extensionRows(q.GetTypesForSchema(ctx, schema)) }},
		{"GetEnumValues", func() (string, error) { return extensionRows(q.GetEnumValues(ctx)) }},
		{"GetEnumValuesForSchema", func() (string, error) { return extensionRows(q.GetEnumValuesForSchema(ctx, schema)) }},
		{"GetCompositeTypeColumns", func() (string, error) { return extensionRows(q.GetCompositeTypeColumns(ctx)) }},
		{"GetCompositeTypeColumnsForSchema", func() (string, error) { return extensionRows(q.GetCompositeTypeColumnsForSchema(ctx, schema)) }},
		{"GetDomains", func() (string, error) { return extensionRows(q.GetDomains(ctx)) }},
		{"GetDomainsForSchema", func() (string, error) { return extensionRows(q.GetDomainsForSchema(ctx, schema)) }},
		{"GetDomainConstraints", func() (string, error) { return extensionRows(q.GetDomainConstraints(ctx)) }},
		{"GetDomainConstraintsForSchema", func() (string, error) { return extensionRows(q.GetDomainConstraintsForSchema(ctx, schema)) }},
		{"GetRLSTables", func() (string, error) { return extensionRows(q.GetRLSTables(ctx)) }},
		{"GetRLSTablesForSchema", func() (string, error) { return extensionRows(q.GetRLSTablesForSchema(ctx, schemaName)) }},
		{"GetRLSPolicies", func() (string, error) { return extensionRows(q.GetRLSPolicies(ctx)) }},
		{"GetRLSPoliciesForSchema", func() (string, error) { return extensionRows(q.GetRLSPoliciesForSchema(ctx, schemaName)) }},
		{"GetTriggers", func() (string, error) { return extensionRows(q.GetTriggers(ctx)) }},
		{"GetTriggersForSchema", func() (string, error) { return extensionRows(q.GetTriggersForSchema(ctx, schema)) }},
		{"GetPartitionChildren", func() (string, error) { return extensionRows(q.GetPartitionChildren(ctx)) }},
		{"GetPartitionedTablesForSchema", func() (string, error) { return extensionRows(q.GetPartitionedTablesForSchema(ctx, schema)) }},
		{"GetPrivilegesForSchema", func() (string, error) { return extensionRows(q.GetPrivilegesForSchema(ctx, schema)) }},
		{"GetRevokedDefaultPrivilegesForSchema", func() (string, error) { return extensionRows(q.GetRevokedDefaultPrivilegesForSchema(ctx, schema)) }},
		{"GetColumnPrivilegesForSchema", func() (string, error) { return extensionRows(q.GetColumnPrivilegesForSchema(ctx, schema)) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := tt.run()
			require.NoError(t, err)
			// Global getters also return our same-name control in the other schema.
			require.NotContains(t, rows, `"member595.Quoted"`)
			// Check member rows by their schema as well as name, so the cross-schema
			// control remains a positive test rather than a name-based ignore.
			var records []map[string]any
			require.NoError(t, json.Unmarshal([]byte(rows), &records))
			for _, record := range records {
				if record["table_schema"] == "extension595_other" {
					continue
				}
				// A managed partition may reference a member parent. Only the
				// child's identity determines whether this row is managed.
				delete(record, "parent_table")
				encoded, err := json.Marshal(record)
				require.NoError(t, err)
				require.NotContains(t, string(encoded), "member595", "extension member leaked")
			}
			require.Contains(t, rows, "app595", "dependent application objects must remain managed")
			switch tt.name {
			case "GetSequences", "GetSequencesForSchema":
				require.Contains(t, rows, "app595_sequence")
				require.Contains(t, rows, "app595_table_id_seq")
			case "GetPrivilegesForSchema":
				require.Contains(t, rows, "app595_table_id_seq")
				require.Contains(t, rows, "app595_view")
				require.Contains(t, rows, "app595_matview")
			case "GetRevokedDefaultPrivilegesForSchema":
				require.Contains(t, rows, "app595_table")
				require.Contains(t, rows, "app595_function")
				require.Contains(t, rows, "app595_procedure")
				require.Contains(t, rows, "app595_enum")
			case "GetPartitionChildren":
				require.Contains(t, rows, "app595_external_partition")
				require.Contains(t, rows, "member595_partitioned")
			}
		})
	}
	rows, err := q.GetTablesForSchema(ctx, sql.NullString{String: "extension595_other", Valid: true})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "member595_table", rows[0].TableName)
}

func extensionRows[T any](rows []T, err error) (string, error) {
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(rows)
	return string(b), err
}

// OIDs are unique only within a catalog. Simulate a collision without waiting
// for an OID wraparound, and roll back every synthetic catalog edge immediately.
// SetupPostgres creates a fresh cluster with a superuser, as required here.
func TestExtensionMembershipCatalogIdentity(t *testing.T) {
	db, _, _, _, _, _ := testutil.ConnectToPostgres(t, sharedTestPostgres)
	defer db.Close()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE FUNCTION public.app595_identity() RETURNS integer LANGUAGE sql AS 'SELECT 1'`)
	require.NoError(t, err)
	defer db.ExecContext(ctx, `DROP FUNCTION public.app595_identity()`)
	for _, tc := range []struct {
		name, class, refclass string
		subid, refsubid       int
	}{
		{"different catalog, same OID", "pg_class", "pg_extension", 0, 0},
		{"different referenced catalog", "pg_proc", "pg_namespace", 0, 0},
		{"different subobject", "pg_proc", "pg_extension", 1, 0},
		{"different referenced subobject", "pg_proc", "pg_extension", 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer tx.Rollback()
			_, err = tx.ExecContext(ctx, `INSERT INTO pg_catalog.pg_depend
    (classid, objid, objsubid, refclassid, refobjid, refobjsubid, deptype)
    VALUES ($1::regclass, 'public.app595_identity()'::regprocedure, $2,
     $3::regclass, (SELECT oid FROM pg_extension WHERE extname='plpgsql'), $4, 'e')`,
				tc.class, tc.subid, tc.refclass, tc.refsubid)
			require.NoError(t, err)
			q := queries.New(tx)
			global, err := extensionRows(q.GetFunctions(ctx))
			require.NoError(t, err)
			require.Contains(t, global, "app595_identity")
			scoped, err := extensionRows(q.GetFunctionsForSchema(ctx, sql.NullString{String: "public", Valid: true}))
			require.NoError(t, err)
			require.Contains(t, scoped, "app595_identity")
		})
	}
}
