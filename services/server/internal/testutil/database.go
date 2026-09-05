// Package testutil provides explicitly owned test resource cleanup.
package testutil

import (
	"testing"

	"gorm.io/gorm"
)

// CloseDB closes a test-owned connection pool after the test's work completes.
// Register this before worker cleanup so workers stop before the database closes.
func CloseDB(t testing.TB, db *gorm.DB) {
	t.Helper()
	pool, err := db.DB()
	if err != nil {
		t.Fatalf("getting test database connection: %v", err)
	}
	t.Cleanup(func() {
		if err := pool.Close(); err != nil {
			t.Errorf("closing test database connection: %v", err)
		}
	})
}
