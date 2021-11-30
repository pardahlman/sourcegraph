package main

import "github.com/sourcegraph/sourcegraph/internal/database/dbconn"

var databases = []*dbconn.Database{
	dbconn.Frontend,
	dbconn.CodeIntel,
	dbconn.CodeInsights,
}

var DatabaseNames []string

func init() {
	for _, database := range databases {
		DatabaseNames = append(DatabaseNames, database.Name)
	}
}
