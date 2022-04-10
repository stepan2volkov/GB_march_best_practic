//go:build integration
// +build integration

package walker

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestDirectoryWalkerIntegration(t *testing.T) {
	logger := zap.NewNop()

	ctx := context.Background()

	want := FileList{
		TargetFile{Name: "main.go", Path: "cmd/findext"},
		TargetFile{Name: "walker.go", Path: "internal/walker"},
		TargetFile{Name: "walker_test.go", Path: "internal/walker"},
		TargetFile{Name: "walker_integration_test.go", Path: "internal/walker"},
	}
	needPrintCurrentStatus := make(<-chan struct{}, 1)
	wg := &sync.WaitGroup{}
	ext := ".go"

	w := &DirectoryWalker{
		logger:                 logger,
		maxDepth:               100,
		needPrintCurrentStatus: needPrintCurrentStatus,
		wg:                     wg,
		readDirFunc:            readDirFunc,
	}

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("error when getting cwd: %v", err)
	}
	dir = filepath.Dir(filepath.Dir(dir))
	t.Logf("current dir: %s\n", dir)
	for i := range want {
		want[i].Path = filepath.Join(dir, want[i].Path, want[i].Name)
	}

	got, err := w.FindFiles(ctx, dir, ext)
	if err != nil {
		t.Fatalf("error when find files: %v", err)
	}

	// Because the order is not guarantee, we should sort it before compare
	sort.Slice(got, func(i int, j int) bool {
		if got[i].Path == got[j].Path {
			return got[i].Name < got[j].Name
		}
		return got[i].Path < got[j].Path
	})
	sort.Slice(want, func(i int, j int) bool {
		if want[i].Path == want[j].Path {
			return want[i].Name < want[j].Name
		}
		return want[i].Path < want[j].Path
	})

	assert.Equal(t, want, got)
}
