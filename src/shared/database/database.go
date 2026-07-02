// Package database — conexión a Postgres (RULE-09: rol de app sin DDL).
//
// El mecanismo de aislamiento RLS fail-closed (fijar app.tenant_id/app.namespace, break-glass
// de system_admin) ya NO vive acá: se migró a github.com/hornosg/go-shared v0.15.0
// (infrastructure/postgres.WithRLSInTransaction + infrastructure/middleware.NamespaceFromContext),
// ver PROP-007. Cada repository abre su propia transacción con esos primitivos — no hay
// conexión fijada por request ni GUC que resetear a mano.
package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

// Connect abre el pool contra lab-postgres usando el rol de app (sin DDL, RULE-09).
func Connect() (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		env("DATABASE_HOST", "lab-postgres"), env("DATABASE_PORT", "5432"),
		env("DATABASE_USER", "notifications_app"), os.Getenv("DATABASE_PASSWORD"),
		env("DATABASE_NAME", "notifications"), env("DATABASE_SSL_MODE", "disable"),
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	return db, db.Ping()
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
