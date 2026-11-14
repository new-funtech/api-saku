package config

import (
	"embed"
	"log"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// Versioned SQL migrations live in sql/NNNN_name.up.sql with a matching .down.sql. The up files are
// applied at startup, in name order; each statement is idempotent (IF NOT EXISTS / IF EXISTS), so
// running them again is harmless. A rollback is done by hand: psql -f sql/NNNN_name.down.sql.
//
//go:embed sql/*.sql
var sqlMigrations embed.FS

func applySQLMigrations(db *gorm.DB) {
	entries, err := sqlMigrations.ReadDir("sql")
	if err != nil {
		log.Printf("Warning: read sql migrations: %v", err)
		return
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		raw, err := sqlMigrations.ReadFile("sql/" + name)
		if err != nil {
			log.Printf("Warning: read %s: %v", name, err)
			continue
		}
		for _, stmt := range splitSQL(string(raw)) {
			if err := db.Exec(stmt).Error; err != nil {
				log.Printf("Warning: migration %s: %v", name, err)
			}
		}
		log.Printf("SQL migration applied: %s", name)
	}
}

// splitSQL splits on ";" at the end of a line and drops "--" comment lines.
func splitSQL(script string) []string {
	var stmts []string
	var cur strings.Builder
	for _, line := range strings.Split(script, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		cur.WriteString(trimmed + " ")
		if strings.HasSuffix(trimmed, ";") {
			stmts = append(stmts, strings.TrimSpace(cur.String()))
			cur.Reset()
		}
	}
	if rest := strings.TrimSpace(cur.String()); rest != "" {
		stmts = append(stmts, rest)
	}
	return stmts
}
