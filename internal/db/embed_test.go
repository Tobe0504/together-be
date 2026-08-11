package db

import (
	"io/fs"
	"strings"
	"testing"
)

// TestMigrationsAreEmbedded guards a failure that is invisible at build time.
//
// The //go:embed directive in db.go looks like a comment. If anything strips
// it — a formatter, an over-eager comment remover, a careless edit —
// migrationsFS silently becomes an empty filesystem. The package still
// compiles, tests that don't touch migrations still pass, the Docker image
// still builds, and the server then dies on its first boot in production
// with "open migrations: file does not exist". That exact sequence has
// already broken one deploy.
//
// This test fails loudly and locally instead.
func TestMigrationsAreEmbedded(t *testing.T) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		t.Fatalf("migrations are not embedded (%v).\n"+
			"The //go:embed directive above `var migrationsFS` in db.go is "+
			"missing or detached from the declaration. It is a compiler "+
			"directive, not a comment — restore it.", err)
	}

	var sqlFiles int
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			sqlFiles++
		}
	}
	if sqlFiles == 0 {
		t.Fatal("migrations directory is embedded but contains no .sql files")
	}
}
