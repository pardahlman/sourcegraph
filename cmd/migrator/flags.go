package main

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/peterbourgon/ff/v3/ffcli"

	"github.com/sourcegraph/sourcegraph/lib/output"
)

var (
	rootFlagSet = flag.NewFlagSet("migrator", flag.ExitOnError)
	rootCommand = &ffcli.Command{
		Name:       "migrator",
		ShortUsage: "migrator <command>",
		ShortHelp:  "Modifies and runs database migrations",
		FlagSet:    rootFlagSet,
		Exec: func(ctx context.Context, args []string) error {
			return flag.ErrHelp
		},
		Subcommands: []*ffcli.Command{
			upCommand,
			downCommand,
		},
	}
)

var (
	upFlagSet          = flag.NewFlagSet("migrator up", flag.ExitOnError)
	upDatabaseNameFlag = upFlagSet.String("db", "all", `The target database instance. Supply "all" (the default) to migrate all databases.`)
	upNFlag            = upFlagSet.Int("n", 0, "How many migrations to apply. Zero (the default) applies all migrations.")
	upCommand          = &ffcli.Command{
		Name:       "up",
		ShortUsage: "migrator up [-db=all] [-n=]",
		ShortHelp:  "Run up migrations",
		FlagSet:    upFlagSet,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				out.WriteLine(output.Linef("", output.StyleWarning, "ERROR: too many arguments"))
				return flag.ErrHelp
			}

			if *upDatabaseNameFlag == "all" && *upNFlag != 0 {
				out.WriteLine(output.Linef("", output.StyleWarning, "ERROR: supply -db to migrate a specific database"))
				return flag.ErrHelp
			}

			var databaseNames []string
			if *upDatabaseNameFlag == "all" {
				databaseNames = append(databaseNames, DatabaseNames...)
			} else {
				databaseNames = append(databaseNames, *upDatabaseNameFlag)
			}

			return run(ctx, runOptions{
				Up:            true,
				NumMigrations: *upNFlag,
				DatabaseNames: databaseNames,
			})
		},
		LongHelp: constructLongHelp(),
	}
)

var (
	downFlagSet          = flag.NewFlagSet("migrator down", flag.ExitOnError)
	downDatabaseNameFlag = downFlagSet.String("db", "", "The target database instance.")
	downNFlag            = downFlagSet.Int("n", 1, "How many migrations to apply.")
	downCommand          = &ffcli.Command{
		Name:       "down",
		ShortUsage: "migrator down -db=... [-n=1]",
		ShortHelp:  "Run down migrations",
		FlagSet:    downFlagSet,
		Exec: func(ctx context.Context, args []string) error {
			if len(args) != 0 {
				out.WriteLine(output.Linef("", output.StyleWarning, "ERROR: too many arguments"))
				return flag.ErrHelp
			}

			if *downDatabaseNameFlag == "" {
				out.WriteLine(output.Linef("", output.StyleWarning, "ERROR: supply -db to migrate a specific database"))
				return flag.ErrHelp
			}

			if *downNFlag == 0 {
				out.WriteLine(output.Linef("", output.StyleWarning, "ERROR: invalid number of migrations"))
				return flag.ErrHelp
			}

			return run(ctx, runOptions{
				Up:            false,
				NumMigrations: *downNFlag,
				DatabaseNames: []string{*downDatabaseNameFlag},
			})
		},
		LongHelp: constructLongHelp(),
	}
)

func constructLongHelp() string {
	names := make([]string, 0, len(DatabaseNames))
	for _, name := range DatabaseNames {
		names = append(names, fmt.Sprintf("  %s", name))
	}

	return fmt.Sprintf("AVAILABLE DATABASES\n%s", strings.Join(names, "\n"))
}
