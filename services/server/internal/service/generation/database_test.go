package generation

import (
	"testing"

	"github.com/mediago-dev/mediago-drama/services/server/internal/repository"
	"github.com/mediago-dev/mediago-drama/services/server/internal/service/media"
	serviceshared "github.com/mediago-dev/mediago-drama/services/server/internal/service/shared"
	"github.com/mediago-dev/mediago-drama/services/server/internal/testutil"
)

func newTestGenerationTaskService(t *testing.T, dbPath string, idGenerator func(string) (string, error)) *GenerationTaskService {
	t.Helper()
	service := NewGenerationTaskService(dbPath, idGenerator)
	if service.initErr != nil {
		return service
	}
	if dbPath == "" {
		dbPath = serviceshared.WorkspacePathsFor("").DatabasePath()
	}
	db, err := repository.OpenWorkspaceDB(dbPath)
	if err != nil {
		t.Fatalf("getting generation test database: %v", err)
	}
	testutil.CloseDB(t, db)
	return service
}

func newTestMediaAssets(t *testing.T, dbPath, mediaDir string) *media.MediaAssets {
	t.Helper()
	assets := media.NewMediaAssets(dbPath, mediaDir)
	db, err := repository.OpenWorkspaceDB(dbPath)
	if err != nil {
		t.Fatalf("getting media test database: %v", err)
	}
	testutil.CloseDB(t, db)
	return assets
}

func newTestGenerationTaskRepository(t *testing.T, dbPath string) (*repository.GenerationTaskRepository, error) {
	t.Helper()
	repos, err := repository.OpenWorkspaceRepositories(dbPath)
	if err != nil {
		return nil, err
	}
	testutil.CloseDB(t, repos.DB)
	return repos.GenerationTasks, nil
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

func newTestGenerationPreferenceService(t *testing.T, dbPath string) *GenerationPreferenceService {
	t.Helper()
	service := NewGenerationPreferenceService(dbPath)
	if service.initErr != nil {
		return service
	}
	db, err := repository.OpenSettingsDB(dbPath)
	if err != nil {
		t.Fatalf("getting preference test database: %v", err)
	}
	testutil.CloseDB(t, db)
	return service
}
