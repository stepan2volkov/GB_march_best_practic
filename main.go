package main

//Исходники задания для первого занятия у других групп https://github.com/t0pep0/GB_best_go

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type TargetFile struct {
	Path string
	Name string
}

type FileList []TargetFile

type FileInfo interface {
	os.FileInfo
	Path() string
}

type fileInfo struct {
	os.FileInfo
	path string
}

func (fi fileInfo) Path() string {
	return fi.path
}

type DirectoryWalker struct {
	logger                 *zap.Logger
	maxDepth               int64
	needPrintCurrentStatus <-chan struct{}
	wg                     *sync.WaitGroup
}

//Ограничить глубину поиска заданым числом, по SIGUSR2 увеличить глубину поиска на +2
func (w *DirectoryWalker) ListDirectory(ctx context.Context, ret chan<- FileInfo, dir string, depth int64) {
	defer w.wg.Done()
	if depth >= w.maxDepth {
		w.logger.Debug("depth is exceed the maximum",
			zap.String("dir", dir),
			zap.Int64("depth", depth))
		return
	}

	res, err := os.ReadDir(dir)
	if err != nil {
		w.logger.Error("error when reading directory",
			zap.String("dir", dir),
			zap.Error(err))
		return
	}

	for _, entry := range res {
		time.Sleep(time.Second)
		select {
		case <-ctx.Done():
			w.logger.Error("context canceled",
				zap.Int64("depth", depth),
				zap.String("dir", dir))
			return
		case <-w.needPrintCurrentStatus:
			fmt.Printf("\tDepth %d from %d.\n\tCWD: %s\n", depth, w.maxDepth, dir)
		default:
			path := filepath.Join(dir, entry.Name())
			w.logger.Debug("scanning directory",
				zap.String("dir", dir))
			if entry.IsDir() {
				w.wg.Add(1)
				go w.ListDirectory(ctx, ret, path, depth+1)
				w.logger.Debug("started handling new dir separtly",
					zap.String("dir", path))
			} else {
				info, err := entry.Info()
				if err != nil {
					w.logger.Error("error when getting info about file",
						zap.String("dir", dir),
						zap.String("filename", entry.Name()),
						zap.String("filename", entry.Name()),
						zap.Error(err))
					continue
				}
				ret <- fileInfo{info, path}
				w.logger.Debug("file handled",
					zap.String("path", path),
					zap.String("filename", entry.Name()))
			}
		}
	}
}

func (w *DirectoryWalker) FindFiles(ctx context.Context, ext string) (FileList, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	files := make(chan FileInfo, 100)

	w.logger.Info("start scanning", zap.String("dir", wd))
	w.wg.Add(1)
	go w.ListDirectory(ctx, files, wd, 0)

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
	w.logger.Info("depth increased",
		zap.Int("current_depth", int(w.maxDepth)))
}

func main() {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder
	fileEncoder := zapcore.NewJSONEncoder(config)
	writer := zapcore.AddSync(os.Stdout)
	defaultLogLevel := zapcore.DebugLevel
	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, writer, defaultLogLevel),
	)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))

	logger.Info("proccess ID", zap.Int("pid", os.Getpid()))
	const wantExt = ".go"

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)

	needPrintCurrentStatus := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}

	walker := DirectoryWalker{
		maxDepth:               5,
		needPrintCurrentStatus: needPrintCurrentStatus,
		wg:                     wg,
		logger:                 logger,
	}

	//Обработать сигнал SIGUSR1
	waitCh := make(chan struct{})
	go func() {
		res, err := walker.FindFiles(ctx, wantExt)
		if err != nil {
			logger.Fatal("error on search", zap.Error(err))
		}
		for _, f := range res {
			fmt.Printf("\tName: %s\t\t Path: %s\n", f.Name, f.Path)
		}
		waitCh <- struct{}{}
	}()
	go func() {
		for {
			switch <-sigCh {
			case syscall.SIGUSR1:
				needPrintCurrentStatus <- struct{}{}
			case syscall.SIGUSR2:
				walker.IncreaseDepth(2)
			default:
				logger.Warn("Signal received, terminate...")
				cancel()
				return
			}
		}
	}()
	//Дополнительно: Ожидание всех горутин перед завершением
	<-waitCh
	logger.Info("Done")
}
