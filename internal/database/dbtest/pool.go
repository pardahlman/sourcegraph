package dbtest

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"net/url"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/hashicorp/go-multierror"
	"github.com/keegancsmith/sqlf"
	"github.com/lib/pq"

	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
)

// testDatabasePool handles creating and reusing migrated database instances
type testDatabasePool struct {
	*basestore.Store
}

const poolSchemaVersion = 1
const poolSchema = `
BEGIN;

CREATE TABLE template_dbs (
	id				bigserial PRIMARY KEY,
	migration_hash  bigint NOT NULL,
	name			text GENERATED ALWAYS AS ('sourcegraph-dbtest-template-' || id::text) STORED,
	created_at		timestamptz DEFAULT now(),
	last_used_at	timestamptz DEFAULT now()
);

CREATE TABLE migrated_dbs (
	id			bigserial PRIMARY KEY,
	template	bigint NOT NULL REFERENCES template_dbs(id) ON DELETE RESTRICT, -- restrict to avoid dangling dbs
	claimed		bool NOT NULL,
	clean		bool NOT NULL,
	name		text GENERATED ALWAYS AS ('sourcegraph-dbtest-migrated-' || id::text) STORED
);

CREATE TABLE schema_version (
	version int NOT NULL
);

INSERT INTO schema_version (version) VALUES (1);
	
COMMIT;
`

func poolSchemaUpToDate(db *sql.DB) bool {
	row := db.QueryRow("SELECT version FROM schema_version")
	var v int
	err := row.Scan(&v)
	if err != nil {
		return false
	}
	return v == poolSchemaVersion
}

func migratePoolDB(db *sql.DB) error {
	_, err := db.Exec(poolSchema)
	return err
}

func (t *testDatabasePool) Transact(ctx context.Context) (*testDatabasePool, error) {
	txBase, err := t.Store.Transact(ctx)
	return &testDatabasePool{Store: txBase}, err
}

type TemplateDB struct {
	ID            int64
	MigrationHash int64
	Name          string
	CreatedAt     time.Time
	LastUsedAt    time.Time
}

var templateDBColumns = []*sqlf.Query{
	sqlf.Sprintf("template_dbs.id"),
	sqlf.Sprintf("template_dbs.migration_hash"),
	sqlf.Sprintf("template_dbs.name"),
	sqlf.Sprintf("template_dbs.created_at"),
	sqlf.Sprintf("template_dbs.last_used_at"),
}

const getTemplateDB = `
UPDATE template_dbs
SET last_used_at = now()
WHERE migration_hash = %s
RETURNING %s
`

const insertTemplateDB = `
INSERT INTO template_dbs (migration_hash)
VALUES (%s)
RETURNING %s
`

// GetTemplate will return a template database that has been migrated with the given migrations.
// The given migrations are hashed and used to identify template databases that have already been
// migrated. If no template database exists with the same hash as the given migrations, a new template
// database is created and the migrations are run.
func (t *testDatabasePool) GetTemplate(ctx context.Context, u *url.URL, defs ...*dbconn.Database) (_ *TemplateDB, err error) {
	// Create a transaction so the exclusive lock is dropped at the end of this function.
	tx, err := t.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	// Create an exclusive lock because we want exactly one template database per hash,
	// and that's difficult to guarantee _and_ guarantee that we don't create the row
	// until the template database is created and fully migrated.
	err = tx.Exec(ctx, sqlf.Sprintf("LOCK TABLE template_dbs IN ACCESS EXCLUSIVE MODE"))
	if err != nil {
		return nil, err
	}

	hash, err := hashMigrations(defs...)
	if err != nil {
		return nil, err
	}

	// Check if the template database has already been created, and return that if it has
	q := sqlf.Sprintf(
		getTemplateDB,
		hash,
		sqlf.Join(templateDBColumns, ","),
	)
	row := tx.QueryRow(ctx, q)
	tdb, err := scanTemplateDB(row)
	if err == nil {
		return tdb, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, errors.Wrap(err, "check if template exists")
	}

	// If the template database has not been created, insert the row to get the
	// generated name, then create the template database below.
	q = sqlf.Sprintf(
		insertTemplateDB,
		hash,
		sqlf.Join(templateDBColumns, ","),
	)
	row = tx.QueryRow(ctx, q)
	tdb, err = scanTemplateDB(row)
	if err != nil {
		return nil, errors.Wrap(err, "insert template row")
	}

	// Create the database outside the transaction (use t.db) because databases
	// cannot be created inside a transaciton. This is safe because the whole
	// template_dbs table is locked in the transaction above, so this
	// will never happen concurrently.
	err = t.Exec(ctx, sqlf.Sprintf("CREATE DATABASE"+pq.QuoteIdentifier(tdb.Name)))
	if err != nil {
		return nil, errors.Wrap(err, "create template database")
	}

	templateDB, err := dbconn.New(dbconn.Opts{DSN: urlWithDB(u, tdb.Name).String()})
	if err != nil {
		return nil, err
	}
	for _, def := range defs {
		done, err := dbconn.MigrateDB(templateDB, def)
		if err != nil {
			return nil, err
		}
		defer done()
	}

	return tdb, nil
}

type MigratedDB struct {
	ID       int64
	Template int64
	Claimed  bool
	Clean    bool
	Name     string
}

var migratedDBColumns = []*sqlf.Query{
	sqlf.Sprintf("migrated_dbs.id"),
	sqlf.Sprintf("migrated_dbs.template"),
	sqlf.Sprintf("migrated_dbs.claimed"),
	sqlf.Sprintf("migrated_dbs.clean"),
	sqlf.Sprintf("migrated_dbs.name"),
}

const insertMigratedDB = `
INSERT INTO migrated_dbs (template, claimed, clean)
VALUES (%s, %s, %s)
RETURNING %s
`

const getExistingMigratedDB = `
UPDATE migrated_dbs
SET claimed = true
WHERE id = (
	SELECT id
	FROM migrated_dbs
	WHERE claimed = false
		AND clean = true
	LIMIT 1
	FOR UPDATE
)
RETURNING %s
`

// GetMigratedDB returns a new, clean, migrated db that is cloned from the given templated db. If an unclaimed,
// clean database already exists for the given template, that is claimed and returned. If it does not, a new
// database is created from the given template and returned.
func (t *testDatabasePool) GetMigratedDB(ctx context.Context, tdb *TemplateDB) (_ *MigratedDB, err error) {
	// Run this in a transaction so if creating the database
	// fails, creating the row is rolled back
	tx, err := t.Transact(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { err = tx.Done(err) }()

	// Check to see if there is a clean, migrated DB already available
	q := sqlf.Sprintf(
		getExistingMigratedDB,
		sqlf.Join(migratedDBColumns, ","),
	)
	row := tx.QueryRow(ctx, q)
	mdb, err := scanMigratedDB(row)
	if err == nil {
		return mdb, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	// Insert a new row, returning the generated name
	q = sqlf.Sprintf(
		insertMigratedDB,
		tdb.ID,
		true,
		false,
		sqlf.Join(migratedDBColumns, ","),
	)
	row = tx.QueryRow(ctx, q)
	mdb, err = scanMigratedDB(row)
	if err != nil {
		return nil, err
	}

	// Create the new database outside of the transaction because databases
	// cannot be created in a transaction.
	err = t.Exec(ctx, sqlf.Sprintf(fmt.Sprintf("CREATE DATABASE %s TEMPLATE %s", pq.QuoteIdentifier(mdb.Name), pq.QuoteIdentifier(tdb.Name))))
	if err != nil {
		return nil, err
	}

	// No need to migrate the new database since it was created from a template
	return mdb, nil
}

const unclaimCleanMigratedDB = `
UPDATE migrated_dbs
SET (claimed, clean) = (false, true)
WHERE id = %s
`

// UnclaimCleanMigratedDB marks a clean database as unclaimed, allowing it to be returned by a
// call to GetMigratedDB. A migrated db should never be unclaimed if it was written to, and should
// be deleted instead. This should really only be called if the database was only used in a transaction
// and that transaction was rolled back (as in NewFastTx).
func (t *testDatabasePool) UnclaimCleanMigratedDB(ctx context.Context, mdb *MigratedDB) error {
	q := sqlf.Sprintf(
		unclaimCleanMigratedDB,
		mdb.ID,
	)
	return t.Exec(ctx, q)
}

const uninsertMigratedDB = `
DELETE FROM migrated_dbs
WHERE id = %s
`

// DeleteMigratedDB deletes a database and untracks it in migrated_dbs. This should
// only be called by the caller who called GetMigratedDB
func (t *testDatabasePool) DeleteMigratedDB(ctx context.Context, mdb *MigratedDB) error {
	err := t.Exec(ctx, sqlf.Sprintf("DROP DATABASE "+pq.QuoteIdentifier(mdb.Name)))
	if err != nil {
		return err
	}

	q := sqlf.Sprintf(uninsertMigratedDB, mdb.ID)
	err = t.Exec(ctx, q)
	return err
}

const listOldTemplateDBs = `
SELECT %s
FROM template_dbs
WHERE 
	migration_hash != %s
	AND last_used_at < NOW() - INTERVAL '1 day'
FOR UPDATE
`

const listMigratedDBsForTemplate = `
SELECT %s
FROM migrated_dbs
WHERE template = %s
FOR UPDATE
`

const uninsertTemplateDB = `
DELETE FROM template_dbs
WHERE id = %s
`

func (t *testDatabasePool) CleanUpOldDBs(ctx context.Context, except ...*dbconn.Database) (err error) {
	hash, err := hashMigrations(except...)
	if err != nil {
		return err
	}

	tx, err := t.Transact(ctx)
	if err != nil {
		return err
	}
	defer func() { err = tx.Done(err) }()

	// List any old template databases that don't have the same
	// hash as the given database definitions
	q := sqlf.Sprintf(
		listOldTemplateDBs,
		sqlf.Join(templateDBColumns, ","),
		hash,
	)
	rows, err := tx.Query(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	oldTDBs, err := scanTemplateDBs(rows)
	if err != nil {
		return err
	}

	var errs *multierror.Error
	for _, tdb := range oldTDBs {
		q = sqlf.Sprintf(
			listMigratedDBsForTemplate,
			tdb.ID,
		)
		rows, err = tx.Query(ctx, q)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}
		defer rows.Close()

		mdbs, err := scanMigratedDBs(rows)
		if err != nil {
			errs = multierror.Append(errs, err)
			continue
		}

		var mdbErrs *multierror.Error
		for _, mdb := range mdbs {
			// Just a best effort delete in case this somehow gets out of sync
			// and that database is already gone
			_ = t.Exec(ctx, sqlf.Sprintf("DROP DATABASE "+pq.QuoteIdentifier(mdb.Name)))

			q := sqlf.Sprintf(uninsertMigratedDB, mdb.ID)
			err = tx.Exec(ctx, q)
			if err != nil {
				mdbErrs = multierror.Append(mdbErrs, err)
			}
		}
		if mdbErrs != nil {
			errs = multierror.Append(mdbErrs)
			continue
		}

		// Just a best effort delete in case this somehow gets out of sync
		// and that database is already gone
		_ = t.Exec(ctx, sqlf.Sprintf("DROP DATABASE "+pq.QuoteIdentifier(tdb.Name)))

		q = sqlf.Sprintf(uninsertTemplateDB, tdb.ID)
		err = tx.Exec(ctx, q)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs.ErrorOrNil()
}

// hashMigrations deterministically hashes all the migrations in the given
// database definitions. This is used to determine whether a new template
// database should be created for the given set of migrations.
func hashMigrations(defs ...*dbconn.Database) (int64, error) {
	hash := fnv.New64()
	for _, def := range defs {
		root, err := def.FS.Open(".")
		if err != nil {
			return 0, err
		}

		rootDir, ok := root.(fs.ReadDirFile)
		if !ok {
			return 0, errors.New("root of migration filesystem is not a directory")
		}

		dirEntries, err := rootDir.ReadDir(0)
		if err != nil {
			return 0, err
		}

		for _, entry := range dirEntries {
			f, err := def.FS.Open(entry.Name())
			if err != nil {
				return 0, err
			}
			_, err = io.Copy(hash, f)
			if err != nil {
				return 0, err
			}
		}
	}
	return int64(hash.Sum64()), nil
}

func scanTemplateDBs(rows *sql.Rows) ([]*TemplateDB, error) {
	var tdbs []*TemplateDB
	for rows.Next() {
		tdb, err := scanTemplateDB(rows)
		if err != nil {
			return nil, err
		}
		tdbs = append(tdbs, tdb)
	}
	return tdbs, nil
}

func scanTemplateDB(scanner dbutil.Scanner) (*TemplateDB, error) {
	var t TemplateDB
	err := scanner.Scan(
		&t.ID,
		&t.MigrationHash,
		&t.Name,
		&t.CreatedAt,
		&t.LastUsedAt,
	)
	return &t, err
}

func scanMigratedDBs(rows *sql.Rows) ([]*MigratedDB, error) {
	var mdbs []*MigratedDB
	for rows.Next() {
		mdb, err := scanMigratedDB(rows)
		if err != nil {
			return nil, err
		}
		mdbs = append(mdbs, mdb)
	}
	return mdbs, nil
}

func scanMigratedDB(scanner dbutil.Scanner) (*MigratedDB, error) {
	var t MigratedDB
	err := scanner.Scan(
		&t.ID,
		&t.Template,
		&t.Claimed,
		&t.Clean,
		&t.Name,
	)
	return &t, err
}
