package batch

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
)

func TestBatchInserter(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	db := dbtesting.GetDB(t)
	setupTestTable(t, db)

	expectedValues := makeTestValues(2, 0)
	testInsert(t, db, expectedValues)

	rows, err := db.Query("SELECT col1, col2, col3, col4, col5 from batch_inserter_test")
	if err != nil {
		t.Fatalf("unexpected error querying data: %s", err)
	}
	defer rows.Close()

	var values [][]interface{}
	for rows.Next() {
		var v1, v2, v3, v4 int
		var v5 string
		if err := rows.Scan(&v1, &v2, &v3, &v4, &v5); err != nil {
			t.Fatalf("unexpected error scanning data: %s", err)
		}

		values = append(values, []interface{}{v1, v2, v3, v4, v5})
	}

	if diff := cmp.Diff(expectedValues, values); diff != "" {
		t.Errorf("unexpected table contents (-want +got):\n%s", diff)
	}
}

func TestBatchInserterWithReturn(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	db := dbtesting.GetDB(t)
	setupTestTable(t, db)

	tableSizeFactor := 2
	numRows := MaxNumPostgresParameters * tableSizeFactor
	expectedValues := makeTestValues(tableSizeFactor, 0)

	var expectedIDs []int
	for i := 0; i < numRows; i++ {
		expectedIDs = append(expectedIDs, i+1)
	}

	if diff := cmp.Diff(expectedIDs, testInsertWithReturn(t, db, expectedValues)); diff != "" {
		t.Errorf("unexpected returned ids (-want +got):\n%s", diff)
	}
}

func TestBatchInserterWithReturnWithConflicts(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	db := dbtesting.GetDB(t)
	setupTestTable(t, db)

	tableSizeFactor := 2
	duplicationFactor := 2
	numRows := MaxNumPostgresParameters * tableSizeFactor
	expectedValues := makeTestValues(tableSizeFactor, 0)

	var expectedIDs []int
	for i := 0; i < numRows; i++ {
		expectedIDs = append(expectedIDs, i+1)
	}

	if diff := cmp.Diff(expectedIDs, testInsertWithReturnWithConflicts(t, db, duplicationFactor, expectedValues)); diff != "" {
		t.Errorf("unexpected returned ids (-want +got):\n%s", diff)
	}
}

func BenchmarkBatchInserter(b *testing.B) {
	db := dbtesting.GetDB(b)
	setupTestTable(b, db)
	expectedValues := makeTestValues(10, 0)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		testInsert(b, db, expectedValues)
	}
}

func BenchmarkBatchInserterLargePayload(b *testing.B) {
	db := dbtesting.GetDB(b)
	setupTestTable(b, db)
	expectedValues := makeTestValues(10, 4096)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		testInsert(b, db, expectedValues)
	}
}

var setup sync.Once

func setupTestTable(t testing.TB, db *sql.DB) {
	setup.Do(func() {
		createTableQuery := `
			CREATE TABLE batch_inserter_test (
				id SERIAL,
				col1 integer NOT NULL UNIQUE,
				col2 integer NOT NULL,
				col3 integer NOT NULL,
				col4 integer NOT NULL,
				col5 text
			)
		`
		if _, err := db.Exec(createTableQuery); err != nil {
			t.Fatalf("unexpected error creating test table: %s", err)
		}
	})
}

func makeTestValues(tableSizeFactor, payloadSize int) [][]interface{} {
	var expectedValues [][]interface{}
	for i := 0; i < MaxNumPostgresParameters*tableSizeFactor; i++ {
		expectedValues = append(expectedValues, []interface{}{
			i,
			i + 1,
			i + 2,
			i + 3,
			makePayload(payloadSize),
		})
	}

	return expectedValues
}

func makePayload(size int) string {
	s := make([]byte, 0, size)
	for i := 0; i < size; i++ {
		s = append(s, '!')
	}

	return string(s)
}

func testInsert(t testing.TB, db *sql.DB, expectedValues [][]interface{}) {
	ctx := context.Background()

	inserter := NewInserter(ctx, db, "batch_inserter_test", MaxNumPostgresParameters, "col1", "col2", "col3", "col4", "col5")
	for _, values := range expectedValues {
		if err := inserter.Insert(ctx, values...); err != nil {
			t.Fatalf("unexpected error inserting values: %s", err)
		}
	}

	if err := inserter.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing: %s", err)
	}
}

func testInsertWithReturn(t testing.TB, db *sql.DB, expectedValues [][]interface{}) (insertedIDs []int) {
	ctx := context.Background()

	inserter := NewInserterWithReturn(
		ctx,
		db,
		"batch_inserter_test",
		MaxNumPostgresParameters,
		[]string{"col1", "col2", "col3", "col4", "col5"},
		"",
		[]string{"id"},
		func(rows *sql.Rows) error {
			var id int
			if err := rows.Scan(&id); err != nil {
				return err
			}

			insertedIDs = append(insertedIDs, id)
			return nil
		},
	)

	for _, values := range expectedValues {
		if err := inserter.Insert(ctx, values...); err != nil {
			t.Fatalf("unexpected error inserting values: %s", err)
		}
	}

	if err := inserter.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing: %s", err)
	}

	return insertedIDs
}

func testInsertWithReturnWithConflicts(t testing.TB, db *sql.DB, n int, expectedValues [][]interface{}) (insertedIDs []int) {
	ctx := context.Background()

	inserter := NewInserterWithReturn(
		ctx,
		db,
		"batch_inserter_test",
		MaxNumPostgresParameters,
		[]string{"id", "col1", "col2", "col3", "col4", "col5"},
		"ON CONFLICT DO NOTHING",
		[]string{"id"},
		func(rows *sql.Rows) error {
			var id int
			if err := rows.Scan(&id); err != nil {
				return err
			}

			insertedIDs = append(insertedIDs, id)
			return nil
		},
	)

	for i := 0; i < n; i++ {
		for j, values := range expectedValues {
			if err := inserter.Insert(ctx, append([]interface{}{j + 1}, values...)...); err != nil {
				t.Fatalf("unexpected error inserting values: %s", err)
			}
		}
	}

	if err := inserter.Flush(ctx); err != nil {
		t.Fatalf("unexpected error flushing: %s", err)
	}

	return insertedIDs
}
