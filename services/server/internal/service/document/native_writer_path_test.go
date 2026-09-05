package document

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSectionSyncPreservesNativeWriterPathAcrossBatches(t *testing.T) {
	for _, filename := range []string{"script.md", "chapters/script.md", "剧本 01.md"} {
		t.Run(filename, func(t *testing.T) {
			store := requireDocumentStore(t)
			projectID := "native-writer-batches"
			projectDir := requireTestProject(t, store, projectID)
			header := "---\ntitle: A different display title\ncategory: screenplay\n---\n\n"
			writeWorkFile(t, projectDir, filename, header)
			if _, err := store.SyncLocalMarkdownFiles(projectID); err != nil {
				t.Fatal(err)
			}
			originalID := docIDByFilename(t, store, projectID, filename)
			expected := ""
			for i := 1; i <= 60; i++ {
				batch := fmt.Sprintf("## Chapter %d\nBATCH_%d\n\n", i, i)
				f, err := os.OpenFile(filepath.Join(projectDir, "work", filename), os.O_APPEND|os.O_WRONLY, 0)
				if err != nil {
					t.Fatalf("batch %d lost original writer path: %v", i, err)
				}
				_, writeErr := f.WriteString(batch)
				closeErr := f.Close()
				if writeErr != nil {
					t.Fatal(writeErr)
				}
				if closeErr != nil {
					t.Fatal(closeErr)
				}
				expected += batch
				if _, err := store.SyncLocalMarkdownFiles(projectID); err != nil {
					t.Fatal(err)
				}
				state, err := store.ListWorkspaceDocuments(projectID)
				if err != nil {
					t.Fatal(err)
				}
				if len(state.Documents) != 1 {
					t.Fatalf("batch %d split document: %d files", i, len(state.Documents))
				}
				doc := state.Documents[0]
				if doc.ID != originalID || doc.Filename != filename || doc.Title != "A different display title" || doc.Category != "screenplay" {
					t.Fatalf("batch %d changed identity: id=%s filename=%s title=%s category=%s", i, doc.ID, doc.Filename, doc.Title, doc.Category)
				}
				lines := []string{}
				for _, line := range strings.Split(doc.Content, "\n") {
					if !documentSectionIDLinePattern.MatchString(line) {
						lines = append(lines, line)
					}
				}
				if strings.TrimSpace(strings.Join(lines, "\n")) != strings.TrimSpace(expected) {
					t.Fatalf("batch %d lost, duplicated or reordered body", i)
				}
			}
		})
	}
}
