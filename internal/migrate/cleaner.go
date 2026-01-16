package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Cleaner removes leftover structures such as invalid indexes or disabled
// triggers while keeping pglogical untouched.
type Cleaner struct{}

func NewCleaner() *Cleaner {
	return &Cleaner{}
}

func (c *Cleaner) Clean(ctx context.Context, dsn string, force bool) error {
	if !force {
		return errors.New("clean requires --force confirmation")
	}

	db, err := openDB(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := dropInvalidIndexes(ctx, tx); err != nil {
		return err
	}
	if err := dropDisabledTriggers(ctx, tx); err != nil {
		return err
	}
	if err := dropTemporaryTables(ctx, tx); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	fmt.Println("✅ Database clean-up completed")
	return nil
}

func dropInvalidIndexes(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
        SELECT quote_ident(n.nspname) || '.' || quote_ident(c.relname)
        FROM pg_index i
        JOIN pg_class c ON c.oid = i.indexrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname NOT IN ('pglogical', 'pg_catalog', 'information_schema')
          AND (NOT i.indisvalid OR NOT i.indisready)
    `)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var identifier string
		if err := rows.Scan(&identifier); err != nil {
			return err
		}
		fmt.Printf("→ Dropping invalid index %s\n", identifier)
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP INDEX IF EXISTS %s", identifier)); err != nil {
			return err
		}
		fmt.Printf("✅ Dropped index %s\n", identifier)
	}

	return rows.Err()
}

func dropDisabledTriggers(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
        SELECT quote_ident(n.nspname) || '.' || quote_ident(rel.relname) AS tbl,
               quote_ident(t.tgname) AS trigger_name
        FROM pg_trigger t
        JOIN pg_class rel ON rel.oid = t.tgrelid
        JOIN pg_namespace n ON n.oid = rel.relnamespace
        WHERE t.tgenabled = 'D'
          AND t.tgisinternal = false
          AND n.nspname NOT IN ('pglogical', 'pg_catalog', 'information_schema')
    `)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var table, trigger string
		if err := rows.Scan(&table, &trigger); err != nil {
			return err
		}
		fmt.Printf("→ Dropping disabled trigger %s on %s\n", trigger, table)
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TRIGGER IF EXISTS %s ON %s", trigger, table)); err != nil {
			return err
		}
		fmt.Printf("✅ Dropped trigger %s on %s\n", trigger, table)
	}

	return rows.Err()
}

func dropTemporaryTables(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
        SELECT quote_ident(table_schema) || '.' || quote_ident(table_name)
        FROM information_schema.tables
        WHERE table_schema NOT IN ('pglogical', 'pg_catalog', 'information_schema')
          AND (table_name LIKE 'tmp_%' OR table_name LIKE 'temp_%' OR table_name LIKE 'backup_%')
    `)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			return err
		}
		fmt.Printf("→ Dropping temporary table %s\n", table)
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)); err != nil {
			return err
		}
		fmt.Printf("✅ Dropped table %s\n", table)
	}

	return rows.Err()
}
