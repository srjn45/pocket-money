package migrations

import "embed"

// FS holds the SQL migration files embedded into the binary so the runtime
// image is self-contained (no on-disk migrations/ directory required). The
// embed directive can only reach files in or under this file's directory, so
// this file lives inside backend/migrations/ alongside the *.sql files.
//
//go:embed *.sql
var FS embed.FS
