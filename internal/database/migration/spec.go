package migration

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sort"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
)

type MigrationSpecs struct {
	migrationSpecs []MigrationSpec
}

func (ms *MigrationSpecs) GetByID(id int) (MigrationSpec, bool) {
	for _, migrationSpec := range ms.migrationSpecs {
		if migrationSpec.ID == id {
			return migrationSpec, true
		}
	}

	return MigrationSpec{}, false
}

func (ms *MigrationSpecs) UpFrom(id, n int) ([]MigrationSpec, error) {
	slice := make([]MigrationSpec, 0, len(ms.migrationSpecs))
	for _, migrationSpec := range ms.migrationSpecs {
		if migrationSpec.ID <= id {
			continue
		}

		slice = append(slice, migrationSpec)
	}

	if n > 0 && len(slice) > n {
		slice = slice[:n]
	}

	if id != 0 && len(slice) != 0 && slice[0].ID != id+1 {
		return nil, errors.Newf("Missing migration (%d, %d)", id+1, slice[0].ID-1)
	}

	return slice, nil
}

func (ms *MigrationSpecs) DownFrom(id, n int) ([]MigrationSpec, error) {
	slice := make([]MigrationSpec, 0, len(ms.migrationSpecs))
	for _, migrationSpec := range ms.migrationSpecs {
		if migrationSpec.ID < id {
			slice = append(slice, migrationSpec)
		}
	}

	sort.Slice(slice, func(i, j int) bool {
		return slice[j].ID < slice[i].ID
	})

	if n > 0 && len(slice) > n {
		slice = slice[:n]
	}

	if id != 0 && len(slice) != 0 && slice[0].ID != id-1 {
		return nil, errors.Newf("Missing migration (%d, %d)", slice[0].ID+1, id-1)
	}

	return slice, nil
}

type MigrationSpec struct {
	ID           int
	UpFilename   string
	UpQuery      *sqlf.Query
	DownFilename string
	DownQuery    *sqlf.Query
}

func ReadMigrationSpecs(fs fs.FS) (*MigrationSpecs, error) {
	filenames, err := readSQLFilenames(fs)
	if err != nil {
		return nil, err
	}

	migrationSpecs, err := buildMigrationSpecStencils(filenames)
	if err != nil {
		return nil, err
	}

	if err := hydrateMigrationSpecs(fs, migrationSpecs); err != nil {
		return nil, err
	}

	return &MigrationSpecs{
		migrationSpecs: migrationSpecs,
	}, nil
}

func readSQLFilenames(fs fs.FS) ([]string, error) {
	root, err := http.FS(fs).Open("/")
	if err != nil {
		return nil, err
	}
	defer func() { _ = root.Close() }()

	files, err := root.Readdir(0)
	if err != nil {
		return nil, err
	}

	filenames := make([]string, 0, len(files))
	for _, file := range files {
		filenames = append(filenames, file.Name())
	}
	sort.Strings(filenames)

	return filenames, nil
}

var pattern = lazyregexp.New(`^(\d+)_[^.]+\.(up|down)\.sql$`)

func buildMigrationSpecStencils(filenames []string) ([]MigrationSpec, error) {
	migrationSpecMap := make(map[int]MigrationSpec, len(filenames))

	// Iterate through the set of filenames looking for things that have the shape
	// of a migration query file. Group these by identifier and match the up and down
	// direction query definitions together.

	for _, filename := range filenames {
		match := pattern.FindStringSubmatch(filename)
		if len(match) == 0 {
			continue
		}

		id, _ := strconv.Atoi(match[1])
		migrationSpec := migrationSpecMap[id]

		if match[2] == "up" {
			// Check for duplicates before overwriting
			if migrationSpec.UpFilename != "" {
				return nil, fmt.Errorf("duplicate upgrade query definition for migration spec %d: %s and %s", id, migrationSpec.UpFilename, filename)
			}

			migrationSpecMap[id] = MigrationSpec{
				UpFilename:   filename,
				DownFilename: migrationSpec.DownFilename,
			}
		} else {
			// Check for duplicates before overwriting
			if migrationSpec.DownFilename != "" {
				return nil, fmt.Errorf("duplicate downgrade query definition for migration spec %d: %s and %s", id, migrationSpec.DownFilename, filename)
			}

			migrationSpecMap[id] = MigrationSpec{
				UpFilename:   migrationSpecMap[id].UpFilename,
				DownFilename: filename,
			}
		}
	}

	// Check for migrations with only direction defined
	// Assign identifiers directly to migration spec values

	for id, migrationSpec := range migrationSpecMap {
		if migrationSpec.UpFilename == "" {
			return nil, fmt.Errorf("upgrade query definition for migration spec %d not found", migrationSpec.ID)
		}
		if migrationSpec.DownFilename == "" {
			return nil, fmt.Errorf("downgrade query definition for migration spec %d not found", migrationSpec.ID)
		}

		migrationSpecMap[id] = MigrationSpec{
			ID:           id,
			UpFilename:   migrationSpec.UpFilename,
			DownFilename: migrationSpec.DownFilename,
		}
	}

	// Flatten migration spec map into ordered list
	migrations := make([]MigrationSpec, 0, len(migrationSpecMap))
	for _, migrationSpec := range migrationSpecMap {
		migrations = append(migrations, migrationSpec)
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].ID < migrations[j].ID
	})

	// Check for gaps in ids
	for i, migrationSpec := range migrations {
		if i > 0 && migrationSpec.ID != migrations[i-1].ID+1 {
			return nil, fmt.Errorf("migration identifiers jump from %d to %d", migrations[i-1].ID, migrationSpec.ID)
		}
	}

	return migrations, nil
}

func hydrateMigrationSpecs(fs fs.FS, migrationSpecs []MigrationSpec) (err error) {
	for i, migrationSpec := range migrationSpecs {
		upQuery, err := readQueryFromFile(fs, migrationSpec.UpFilename)
		if err != nil {
			return err
		}

		downQuery, err := readQueryFromFile(fs, migrationSpec.DownFilename)
		if err != nil {
			return err
		}

		migrationSpecs[i] = MigrationSpec{
			ID:           migrationSpec.ID,
			UpFilename:   migrationSpec.UpFilename,
			UpQuery:      upQuery,
			DownFilename: migrationSpec.DownFilename,
			DownQuery:    downQuery,
		}
	}

	return nil
}

func readQueryFromFile(fs fs.FS, filepath string) (*sqlf.Query, error) {
	file, err := fs.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	contents, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return sqlf.Sprintf(string(contents)), nil
}
