package main

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/inconshreveable/log15"
	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/migration"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/trace"
)

type runOptions struct {
	NumMigrations int
	Up            bool
	DatabaseNames []string
}

func run(ctx context.Context, options runOptions) error {
	observationContext := &observation.Context{
		Logger:     log15.Root(),
		Tracer:     &trace.Tracer{Tracer: opentracing.GlobalTracer()},
		Registerer: prometheus.DefaultRegisterer,
	}
	operations := migration.NewOperations(observationContext)

	//
	// TODO - split into stages
	//

	for _, databaseName := range options.DatabaseNames {
		log15.Info("Migrating", "databaseName", databaseName)

		var db *dbconn.Database
		for _, database := range databases {
			if database.Name == databaseName {
				db = database
			}
		}
		if db == nil {
			return errors.Newf("unknown database '%s'", databaseName)
		}

		// TODO - this dsn will also differ
		opts := dbconn.Opts{DSN: "postgres://sourcegraph@localhost:5432/sourcegraph", DBName: databaseName, AppName: "migrator"}
		store, err := initializeStore(ctx, opts, db.MigrationsTable, operations)
		if err != nil {
			return err
		}

		version, ok, err := store.Version(ctx)
		if err != nil {
			return err
		}

		log15.Info("Checked current version", "databaseName", databaseName, "version", version)

		if !ok && !options.Up {
			return errors.New("Cannot downgrade fresh database")
		}

		migrationSpecs, err := migration.ReadMigrationSpecs(db.FS)
		if err != nil {
			return err
		}

		if options.Up {
			migrations, err := migrationSpecs.UpFrom(version, options.NumMigrations)
			if err != nil {
				return err
			}

			for _, migration := range migrations {
				log15.Info("Running up migration", "databaseName", databaseName, "migrationID", migration.ID)

				if store.Up(ctx, migration); err != nil {
					return err
				}
			}
		} else {
			migrations, err := migrationSpecs.DownFrom(version, options.NumMigrations)
			if err != nil {
				return err
			}

			for _, migration := range migrations {
				log15.Info("Running down migration", "databaseName", databaseName, "migrationID", migration.ID)

				if store.Down(ctx, migration); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func initializeStore(ctx context.Context, opts dbconn.Opts, migrationsTable string, operations *migration.Operations) (*migration.Store, error) {
	db, err := dbconn.New(opts)
	if err != nil {
		return nil, err
	}

	store := migration.NewWithDB(
		db,
		migrationsTable,
		operations,
	)

	if err := store.EnsureSchemaTable(ctx); err != nil {
		return nil, err
	}

	return store, nil
}
