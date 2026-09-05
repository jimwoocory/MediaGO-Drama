package app

import (
	"io/fs"
	"net/http"
	"os"
	"testing"

	"github.com/mediago-dev/mediago-drama/services/server/internal/repository"
	"github.com/mediago-dev/mediago-drama/services/server/internal/testutil"
)

func newTestHandlerWithConfig(t *testing.T, staticFS fs.FS, config Config) http.Handler {
	t.Helper()
	handler := NewHandlerWithConfig(staticFS, config)
	appHandler := handler.(*Handler)
	if appHandler.api != nil && appHandler.api.workspaceState != nil {
		workspace := appHandler.api.workspaceState
		settingsPath := config.SettingsDBPath
		if settingsPath == "" {
			settingsPath = workspace.SettingsDatabasePath()
		}
		for _, dbPath := range []string{workspace.DatabasePath(), settingsPath} {
			// Failed-initialization tests may intentionally not create a database.
			if _, err := os.Stat(dbPath); os.IsNotExist(err) {
				continue
			}
			db, err := repository.OpenGormSQLite(dbPath)
			if err != nil {
				t.Fatalf("getting application test database: %v", err)
			}
			testutil.CloseDB(t, db)
		}
	}
	// Registered last: workers and buffered writers stop before DB pools close.
	closeTestHandler(t, handler)
	return handler
}
