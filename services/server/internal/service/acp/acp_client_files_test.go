package acp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	acpsdk "github.com/coder/acp-go-sdk"
)

func TestACPDirectoryLinkBoundary(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	outside := filepath.Join(root, "outside")
	for _, dir := range []string{project, outside} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	sentinel := filepath.Join(outside, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("AUDIT_OUTSIDE_FIXTURE"), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(project, "linked")
	if runtime.GOOS == "windows" {
		if output, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
			t.Fatalf("fixture junction: %v %s", err, output)
		}
	} else if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	client := &acpClient{workspaceDir: project}
	path := filepath.Join(link, "sentinel.txt")
	_, err := client.ReadTextFile(context.Background(), acpsdk.ReadTextFileRequest{Path: path})
	if err == nil {
		t.Error("ACP read escaped workspace through directory link")
	}
	_, err = client.WriteTextFile(context.Background(), acpsdk.WriteTextFileRequest{Path: path, Content: "AUDIT_CHANGED"})
	content, readErr := os.ReadFile(sentinel)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if err == nil || string(content) != "AUDIT_OUTSIDE_FIXTURE" {
		t.Error("ACP write escaped workspace through directory link")
	}
}

func TestACPFileBoundaryAllowsNestedWorkspaceFiles(t *testing.T) {
	project := t.TempDir()
	client := &acpClient{workspaceDir: project}
	path := filepath.Join(project, "剧本", "分段 文稿.md")
	want := "第一段\n第二段\n"
	if _, err := client.WriteTextFile(context.Background(), acpsdk.WriteTextFileRequest{Path: path, Content: want}); err != nil {
		t.Fatalf("writing nested workspace file: %v", err)
	}
	got, err := client.ReadTextFile(context.Background(), acpsdk.ReadTextFileRequest{Path: path})
	if err != nil || got.Content != want {
		t.Fatalf("reading nested workspace file: %q, %v", got.Content, err)
	}
}

func TestACPFileBoundaryRejectsInvalidRootsAndPaths(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, workspace, path string
	}{
		{"outside", project, filepath.Join(root, "outside.md")},
		{"relative", project, "relative.md"},
		{"missing-root", "", filepath.Join(project, "file.md")},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &acpClient{workspaceDir: test.workspace}
			if _, err := client.ReadTextFile(context.Background(), acpsdk.ReadTextFileRequest{Path: test.path}); err == nil {
				t.Error("read accepted invalid workspace/path")
			}
			if _, err := client.WriteTextFile(context.Background(), acpsdk.WriteTextFileRequest{Path: test.path, Content: "denied"}); err == nil {
				t.Error("write accepted invalid workspace/path")
			}
		})
	}
}
