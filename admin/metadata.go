package admin

import (
	stdcontext "context"
	"database/sql"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	goadminctx "github.com/GoAdminGroup/go-admin/context"
	"github.com/GoAdminGroup/go-admin/modules/db"
	"github.com/GoAdminGroup/go-admin/plugins/admin/modules/table"
	"github.com/GoAdminGroup/go-admin/template/types/form"
)

type columnMeta struct {
	Name       string
	DataType   string
	UDTName    string
	IsPrimary  bool
	IsNullable bool
}

type tableMeta struct {
	Name      string
	Primary   columnMeta
	Columns   []columnMeta
	Generator table.Generator
}

func discoverTables(dbConn *sql.DB) ([]tableMeta, error) {
	ctx, cancel := stdcontext.WithTimeout(stdcontext.Background(), 5*time.Second)
	defer cancel()

	rows, err := dbConn.QueryContext(ctx, `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND (
			 table_type = 'BASE TABLE'
			 OR (table_type = 'VIEW' AND table_name LIKE 'admin_%')
		  )
		  AND table_name NOT LIKE 'goadmin_%'
		  AND table_name <> 'schema_migrations'
		ORDER BY table_name
	`)
	if err != nil {
		return nil, fmt.Errorf("query tables for admin: %w", err)
	}
	defer rows.Close()

	tables := make([]tableMeta, 0)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("scan table name: %w", err)
		}

		meta, err := discoverColumns(ctx, dbConn, tableName)
		if err != nil {
			return nil, err
		}
		tables = append(tables, meta)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tables: %w", err)
	}

	return tables, nil
}

func discoverColumns(ctx stdcontext.Context, dbConn *sql.DB, tableName string) (tableMeta, error) {
	rows, err := dbConn.QueryContext(ctx, `
		SELECT
			c.column_name,
			c.data_type,
			c.udt_name,
			c.is_nullable = 'YES' AS is_nullable,
			EXISTS (
				SELECT 1
				FROM information_schema.table_constraints tc
				JOIN information_schema.key_column_usage kcu
					ON tc.constraint_name = kcu.constraint_name
					AND tc.table_schema = kcu.table_schema
				WHERE tc.table_schema = 'public'
					AND tc.table_name = c.table_name
					AND tc.constraint_type = 'PRIMARY KEY'
					AND kcu.column_name = c.column_name
				) AS is_primary
		FROM information_schema.columns c
		WHERE c.table_schema = 'public'
		  AND c.table_name = $1
		ORDER BY c.ordinal_position
	`, tableName)
	if err != nil {
		return tableMeta{}, fmt.Errorf("query columns for table %s: %w", tableName, err)
	}
	defer rows.Close()

	meta := tableMeta{Name: tableName}
	for rows.Next() {
		var col columnMeta
		if err := rows.Scan(&col.Name, &col.DataType, &col.UDTName, &col.IsNullable, &col.IsPrimary); err != nil {
			return tableMeta{}, fmt.Errorf("scan column for table %s: %w", tableName, err)
		}
		meta.Columns = append(meta.Columns, col)
		if col.IsPrimary {
			meta.Primary = col
		}
	}
	if err := rows.Err(); err != nil {
		return tableMeta{}, fmt.Errorf("iterate columns for table %s: %w", tableName, err)
	}

	if len(meta.Columns) == 0 {
		return tableMeta{}, fmt.Errorf("table %s has no columns", tableName)
	}

	if meta.Primary.Name == "" {
		meta.Primary = meta.Columns[0]
	}

	return meta, nil
}

func buildTable(ctx *goadminctx.Context, meta tableMeta, appDB *sql.DB) table.Table {
	tb := table.NewDefaultTable(
		ctx,
		table.DefaultConfigWithDriver(driverName).
			SetPrimaryKey(meta.Primary.Name, mapDBType(meta.Primary)),
	)

	info := tb.GetInfo()
	for _, col := range meta.Columns {
		info.AddField(toLabel(col.Name), col.Name, mapDBType(col)).FieldFilterable()
	}
	info.SetTable(meta.Name).SetTitle(toLabel(meta.Name)).SetDescription("Auto-generated admin CRUD")

	if meta.Name == "admin_payment_subscribers_v" {
		info.HideNewButton().HideEditButton().HideDeleteButton()
	}

	if meta.Name == "admin_payment_requests_v" {
		info.HideNewButton().HideEditButton().SetDeleteFn(func(idArr []string) error {
			return deletePendingPaymentsByIDs(appDB, idArr)
		})
	}

	if meta.Name == "subscriptions" {
		info.HideNewButton().HideEditButton().HideDeleteButton().SetDeleteFn(func(idArr []string) error {
			return fmt.Errorf("deleting subscriptions from admin panel is disabled")
		})
	}

	formList := tb.GetForm()
	for _, col := range meta.Columns {
		field := formList.AddField(toLabel(col.Name), col.Name, mapDBType(col), mapFormType(col))
		if col.IsPrimary {
			field.FieldDisplayButCanNotEditWhenUpdate().FieldDisableWhenCreate()
		}
	}
	formList.SetTable(meta.Name).SetTitle(toLabel(meta.Name)).SetDescription("Auto-generated admin form")

	return tb
}

func isPaymentPanelTable(name string) bool {
	switch name {
	case "subscriptions", "payments", "admin_payment_subscribers_v", "admin_payment_requests_v":
		return true
	default:
		return false
	}
}

func mapDBType(col columnMeta) db.DatabaseType {
	dataType := strings.ToLower(strings.TrimSpace(col.DataType))
	udtName := strings.ToLower(strings.TrimSpace(col.UDTName))

	switch dataType {
	case "smallint":
		return db.Smallint
	case "integer":
		return db.Int4
	case "bigint":
		return db.Bigint
	case "numeric", "decimal":
		return db.Numeric
	case "real":
		return db.Real
	case "double precision":
		return db.Doubleprecision
	case "boolean":
		return db.Bool
	case "date":
		return db.Date
	case "timestamp with time zone", "timestamp without time zone":
		return db.Timestamp
	case "json", "jsonb":
		return db.JSON
	case "uuid":
		return db.UUID
	case "character varying", "character", "text":
		return db.Varchar
	}

	switch udtName {
	case "int2":
		return db.Int2
	case "int4":
		return db.Int4
	case "int8":
		return db.Int8
	case "uuid":
		return db.UUID
	case "json", "jsonb":
		return db.JSON
	case "bool":
		return db.Bool
	case "timestamptz", "timestamp":
		return db.Timestamp
	}

	return db.Varchar
}

func mapFormType(col columnMeta) form.Type {
	name := strings.ToLower(col.Name)
	typ := mapDBType(col)

	switch typ {
	case db.Bool, db.Boolean:
		return form.Switch
	case db.Timestamp, db.Datetime, db.Timestamptz, db.Date:
		return form.Datetime
	case db.Text, db.Longtext, db.Mediumtext, db.JSON:
		return form.TextArea
	}

	if strings.HasSuffix(name, "_json") || strings.Contains(name, "json") {
		return form.TextArea
	}

	return form.Text
}

func toLabel(name string) string {
	parts := strings.Split(strings.ReplaceAll(name, "-", "_"), "_")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + strings.ToLower(parts[i][1:])
	}
	return strings.Join(parts, " ")
}

func AdminURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "/" + urlPrefix
	}

	parsed, err := url.Parse(baseURL)
	if err != nil {
		return strings.TrimRight(baseURL, "/") + "/" + urlPrefix
	}

	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/" + urlPrefix
	return parsed.String()
}

func AvailableTableNames(dbConn *sql.DB) ([]string, error) {
	tables, err := discoverTables(dbConn)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(tables))
	for _, t := range tables {
		names = append(names, t.Name)
	}
	sort.Strings(names)
	return names, nil
}
