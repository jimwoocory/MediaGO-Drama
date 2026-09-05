package media

import (
	"testing"

	"github.com/mediago-dev/mediago-drama/services/server/internal/repository"
	"github.com/mediago-dev/mediago-drama/services/server/internal/service/shared"
	"github.com/mediago-dev/mediago-drama/services/server/internal/testutil"
)

func newTestMediaAssets(t *testing.T, dbPath, mediaDir string) *MediaAssets {
	t.Helper()
	store := NewMediaAssets(dbPath, mediaDir)
	if store.initErr == nil {
		if dbPath == "" {
			dbPath = shared.WorkspacePathsFor("").DatabasePath()
		}
		db, err := repository.OpenWorkspaceDB(dbPath)
		if err != nil {
			t.Fatal(err)
		}
		testutil.CloseDB(t, db)
	}
	return store
}

func newTestMediaAssetRepository(t *testing.T, dbPath string) (*repository.MediaAssetRepository, error) {
	t.Helper()
	repos, err := repository.OpenWorkspaceRepositories(dbPath)
	if err != nil {
		return nil, err
	}
	testutil.CloseDB(t, repos.DB)
	return repos.MediaAssets, nil
}
