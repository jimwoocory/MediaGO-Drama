package mcp

import (
	"os"
	"path/filepath"
	"testing"

	appworkspace "github.com/mediago-dev/mediago-drama/services/server/internal/app/workspace"
	"github.com/mediago-dev/mediago-drama/services/server/internal/repository"
	cliservice "github.com/mediago-dev/mediago-drama/services/server/internal/service/document"
	"github.com/mediago-dev/mediago-drama/services/server/internal/testutil"
)

type workspaceStateService = appworkspace.WorkspaceStateService
type createWorkspaceDocumentRequest = cliservice.CreateWorkspaceDocumentRequest

func newWorkspaceStateService(t *testing.T, workspaceDir string) *workspaceStateService {
	t.Helper()
	store := appworkspace.NewStateService(workspaceDir)
	db, err := repository.OpenWorkspaceDB(store.DatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	testutil.CloseDB(t, db)
	settingsDB, err := repository.OpenGormSQLite(store.SettingsDatabasePath())
	if err != nil {
		t.Fatal(err)
	}
	testutil.CloseDB(t, settingsDB)
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("closing MCP test workspace: %v", err)
		}
	})
	return store
}

func requireMCPTestProject(t *testing.T, store *workspaceStateService, projectID string) string {
	t.Helper()
	projectDir := filepath.Join(t.TempDir(), projectID)
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("closing MCP project writers: %v", err)
		}
	})
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("creating project dir: %v", err)
	}
	if _, err := store.StateService().Documents.CreateProject(projectID, cliservice.CreateWorkspaceProjectRequest{
		Name:       projectID,
		ProjectDir: projectDir,
	}); err != nil {
		t.Fatalf("creating project %s: %v", projectID, err)
	}
	return projectDir
}

func mcpTestProjectDir(t *testing.T, store *workspaceStateService, projectID string) string {
	t.Helper()
	projectDir, err := store.StateService().Documents.ProjectDir(projectID)
	if err != nil {
		t.Fatalf("reading project dir: %v", err)
	}
	return projectDir
}
