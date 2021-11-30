package migration

import (
	"context"
	"database/sql"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type Store struct {
	*basestore.Store
	migrationsTable string
	operations      *Operations
}

func NewWithDB(db dbutil.DB, migrationsTable string, operations *Operations) *Store {
	return &Store{
		Store:           basestore.NewWithDB(db, sql.TxOptions{}),
		migrationsTable: migrationsTable,
		operations:      operations,
	}
}

func (s *Store) With(other basestore.ShareableStore) *Store {
	return &Store{
		Store:           s.Store.With(other),
		migrationsTable: s.migrationsTable,
		operations:      s.operations,
	}
}

func (s *Store) Transact(ctx context.Context) (*Store, error) {
	txBase, err := s.Store.Transact(ctx)
	if err != nil {
		return nil, err
	}

	return &Store{
		Store:           txBase,
		migrationsTable: s.migrationsTable,
		operations:      s.operations,
	}, nil
}

func (s *Store) EnsureSchemaTable(ctx context.Context) (err error) {
	ctx, endObservation := s.operations.ensureSchemaTable.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	return s.Exec(ctx, sqlf.Sprintf(ensureSchemaTableQuery, quote(s.migrationsTable)))
}

const ensureSchemaTableQuery = `
-- source: internal/database/migration/store.go:EnsureSchemaTable
CREATE TABLE IF NOT EXISTS %s (version bigint NOT NULL PRIMARY KEY, dirty boolean NOT NULL);
`

func (s *Store) Version(ctx context.Context) (_ int, _ bool, err error) {
	ctx, endObservation := s.operations.version.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	return basestore.ScanFirstInt(s.Query(ctx, sqlf.Sprintf(versionQuery, quote(s.migrationsTable))))
}

const versionQuery = `
-- source: internal/database/migration/store.go:Version
SELECT version FROM %s
`

func (s *Store) Up(ctx context.Context, migrationSpec MigrationSpec) (err error) {
	ctx, endObservation := s.operations.up.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	return s.runSpec(ctx, migrationSpec, migrationSpec.UpQuery)
}

func (s *Store) Down(ctx context.Context, migrationSpec MigrationSpec) (err error) {
	ctx, endObservation := s.operations.down.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	return s.runSpec(ctx, migrationSpec, migrationSpec.DownQuery)
}

func (s *Store) runSpec(ctx context.Context, migrationSpec MigrationSpec, query *sqlf.Query) error {
	if err := s.setVersion(ctx, migrationSpec.ID); err != nil {
		return err
	}

	if err := s.Exec(ctx, query); err != nil {
		return err
	}

	if err := s.Exec(ctx, sqlf.Sprintf(`UPDATE %s SET dirty=false`, quote(s.migrationsTable))); err != nil {
		return err
	}

	return nil
}

func (s *Store) setVersion(ctx context.Context, version int) (err error) {
	tx, err := s.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	if err := tx.Exec(ctx, sqlf.Sprintf(`DELETE FROM %s`, quote(s.migrationsTable))); err != nil {
		return err
	}

	if err := tx.Exec(ctx, sqlf.Sprintf(`INSERT INTO %s (version, dirty) VALUES (%s, false)`, quote(s.migrationsTable), version)); err != nil {
		return err
	}

	return nil
}

var quote = sqlf.Sprintf
