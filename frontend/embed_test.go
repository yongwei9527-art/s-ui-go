package frontend

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestEveryBuiltAssetIsEmbedded(t *testing.T) {
	err := filepath.WalkDir("dist/assets", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		embedded, err := Content.ReadFile(filepath.ToSlash(path))
		if err != nil {
			t.Errorf("built asset missing from executable: %s: %v", path, err)
			return nil
		}
		if string(source) != string(embedded) {
			t.Errorf("embedded asset differs from build: %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
