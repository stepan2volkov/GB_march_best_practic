package walker

import (
	"context"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

type fileReaderStub struct {
	files map[string][]FileInfo
}

func newFileReaderStub(files []string) fileReaderStub {
	dirMap := map[string]struct{}{}

	fileMap := make(map[string][]FileInfo, len(files))

	for _, file := range files {
		dir, filename := filepath.Split(file)
		dir = filepath.Clean(dir)
		fileMap[dir] = append(fileMap[dir], fileInfo{name: filename, path: dir})

		if parentDir := filepath.Dir(dir); parentDir != dir {
			if _, found := dirMap[dir]; found {
				continue
			}
			dirMap[dir] = struct{}{}
			_, filename = filepath.Split(dir)
			fileMap[parentDir] = append(fileMap[parentDir], fileInfo{name: filename, path: parentDir, isDir: true})
		}
	}

	return fileReaderStub{files: fileMap}
}

func (f fileReaderStub) ReadDir(dir string) ([]FileInfo, error) {
	return f.files[dir], nil
}

func TestDirectoryWalker_FindFiles(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("error when creating logger: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name                   string
		maxDepth               int64
		needPrintCurrentStatus <-chan struct{}
		wg                     *sync.WaitGroup
		files                  []string
		dir                    string
		ext                    string
		want                   FileList
	}{
		{
			name:                   "three files",
			maxDepth:               100,
			needPrintCurrentStatus: make(<-chan struct{}, 1),
			wg:                     &sync.WaitGroup{},
			files: []string{
				"/folder/file1.md",
				"/folder/file2.md",
				"/folder/file3.md",
			},
			dir: "/folder",
			ext: ".md",
			want: FileList{
				TargetFile{Name: "file1.md", Path: "/folder"},
				TargetFile{Name: "file3.md", Path: "/folder"},
				TargetFile{Name: "file2.md", Path: "/folder"},
			},
		},
		{
			name:                   "two subfolders",
			maxDepth:               100,
			needPrintCurrentStatus: make(<-chan struct{}, 1),
			wg:                     &sync.WaitGroup{},
			files: []string{
				"/folder/sub1/file1.md",
				"/folder/sub2/file2.md",
				"/folder/file3.md",
				"/folder/file4.go",
			},
			dir: "/folder",
			ext: ".md",
			want: FileList{
				TargetFile{Name: "file1.md", Path: "/folder/sub1"},
				TargetFile{Name: "file2.md", Path: "/folder/sub2"},
				TargetFile{Name: "file3.md", Path: "/folder"},
			},
		},
		{
			name:                   "no files",
			maxDepth:               100,
			needPrintCurrentStatus: make(<-chan struct{}, 1),
			wg:                     &sync.WaitGroup{},
			files: []string{
				"/folder/file1.md",
				"/folder/file2.md",
				"/folder/file3.md",
			},
			dir: "/folder",
			ext: ".go",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &DirectoryWalker{
				logger:                 logger,
				maxDepth:               tt.maxDepth,
				needPrintCurrentStatus: tt.needPrintCurrentStatus,
				wg:                     tt.wg,
			}
			stub := newFileReaderStub(tt.files)
			w.readDirFunc = stub.ReadDir

			got, err := w.FindFiles(ctx, tt.dir, tt.ext)
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
			sort.Slice(tt.want, func(i int, j int) bool {
				if tt.want[i].Path == tt.want[j].Path {
					return tt.want[i].Name < tt.want[j].Name
				}
				return tt.want[i].Path < tt.want[j].Path
			})

			assert.Equal(t, tt.want, got)
		})
	}
}
