package walker

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

type TargetFile struct {
	Path string
	Name string
}

type FileList []TargetFile

type FileInfo interface {
	Name() string
	Path() string
	IsDir() bool
}

type fileInfo struct {
	name  string
	path  string
	isDir bool
}

func (fi fileInfo) Name() string {
	return fi.name
}

func (fi fileInfo) Path() string {
	return fi.path
}

func (fi fileInfo) IsDir() bool {
	return fi.isDir
}

func readDirFunc(dir string) ([]FileInfo, error) {
	res, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	ret := make([]FileInfo, 0, len(res))
	for _, dirEntry := range res {
		ret = append(ret, fileInfo{
			name:  dirEntry.Name(),
			path:  filepath.Join(dir, dirEntry.Name()),
			isDir: dirEntry.IsDir(),
		})
	}

	return ret, nil
}

func New(logger *zap.Logger, maxDepth int64, printChan <-chan struct{}, wg *sync.WaitGroup) *DirectoryWalker {
	return &DirectoryWalker{
		logger:                 logger,
		maxDepth:               maxDepth,
		needPrintCurrentStatus: printChan,
		wg:                     wg,
		readDirFunc:            readDirFunc,
	}
}

type DirectoryWalker struct {
	logger                 *zap.Logger
	maxDepth               int64
	needPrintCurrentStatus <-chan struct{}
	wg                     *sync.WaitGroup
	readDirFunc            func(name string) ([]FileInfo, error)
}

func (w *DirectoryWalker) listDirectory(ctx context.Context, ret chan<- FileInfo, dir string, depth int64) {
	defer w.wg.Done()
	if depth >= w.maxDepth {
		w.logger.Debug("depth is exceed the maximum",
			zap.String("dir", dir),
			zap.Int64("depth", depth))
		return
	}

	res, err := w.readDirFunc(dir)
	if err != nil {
		w.logger.Error("error when reading directory",
			zap.String("dir", dir),
			zap.Error(err))
		return
	}

	for _, entry := range res {
		// time.Sleep(time.Second)
		select {
		case <-ctx.Done():
			w.logger.Info("context canceled",
				zap.Int64("depth", depth),
				zap.String("dir", dir))
			return
		case <-w.needPrintCurrentStatus:
			w.logger.Info("signal for printing cwd recieved",
				zap.Int64("depth", depth),
				zap.Int64("max_depth", w.maxDepth),
				zap.String("dir", dir))
		default:
			path := filepath.Join(dir, entry.Name())
			w.logger.Debug("scanning directory", zap.String("dir", dir))

			if entry.IsDir() {
				w.wg.Add(1)
				go w.listDirectory(ctx, ret, path, depth+1)
				w.logger.Debug("started handling new dir separtly", zap.String("dir", path))
			} else {
				ret <- entry
				w.logger.Debug("file handled",
					zap.String("path", path),
					zap.String("filename", entry.Name()))
			}
		}
	}
}

func (w *DirectoryWalker) FindFiles(ctx context.Context, dir string, ext string) (FileList, error) {
	files := make(chan FileInfo, 100)

	w.logger.Info("start scanning", zap.String("dir", dir))
	w.wg.Add(1)
	go w.listDirectory(ctx, files, dir, 0)

	go func() {
		w.wg.Wait()
		close(files)
	}()
	var ret FileList
	for file := range files {
		if filepath.Ext(file.Name()) == ext {
			w.logger.Debug("extension found",
				zap.String("path", file.Path()),
				zap.String("filename", file.Name()))
			ret = append(ret, TargetFile{Name: file.Name(), Path: file.Path()})
		}
	}
	return ret, nil
}

func (w *DirectoryWalker) IncreaseDepth(delta int) {
	atomic.AddInt64(&w.maxDepth, int64(delta))
	w.logger.Info("depth increased", zap.Int("current_depth", int(w.maxDepth)))
}
