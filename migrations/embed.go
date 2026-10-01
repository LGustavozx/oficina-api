// Package migrations embute os arquivos SQL de migration no binário.
package migrations

import "embed"

// FS contém os arquivos *.sql versionados.
//
//go:embed *.sql
var FS embed.FS
