package main

//Исходники задания для первого занятия у других групп https://github.com/t0pep0/GB_best_go

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"
)

type TargetFile struct {
	Path string
	Name string
}

type FileList map[string]TargetFile

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
	maxDepth int64
}

//Ограничить глубину поиска заданым числом, по SIGUSR2 увеличить глубину поиска на +2
func (w *DirectoryWalker) ListDirectory(ctx context.Context, dir string) ([]FileInfo, error) {
	select {
	case <-ctx.Done():
		return nil, nil
	default:
		//По SIGUSR1 вывести текущую директорию и текущую глубину поиска
		time.Sleep(time.Second * 10)
		var result []FileInfo
		res, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, entry := range res {
			path := filepath.Join(dir, entry.Name())
			if entry.IsDir() {
				child, err := w.ListDirectory(ctx, path) //Дополнительно: вынести в горутину
				if err != nil {
					return nil, err
				}
				result = append(result, child...)
			} else {
				info, err := entry.Info()
				if err != nil {
					return nil, err
				}
				result = append(result, fileInfo{info, path})
			}
		}
		return result, nil
	}
}

func (w *DirectoryWalker) FindFiles(ctx context.Context, ext string) (FileList, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	files, err := w.ListDirectory(ctx, wd)
	if err != nil {
		return nil, err
	}
	fl := make(FileList, len(files))
	for _, file := range files {
		if filepath.Ext(file.Name()) == ext {
			fl[file.Name()] = TargetFile{
				Name: file.Name(),
				Path: file.Path(),
			}
		}
	}
	return fl, nil
}

func (w *DirectoryWalker) IncreaseDepth(delta int) {
	atomic.AddInt64(&w.maxDepth, int64(delta))
}

func main() {
	const wantExt = ".go"
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2)

	walker := DirectoryWalker{maxDepth: 5}

	//Обработать сигнал SIGUSR1
	waitCh := make(chan struct{})
	go func() {
		res, err := walker.FindFiles(ctx, wantExt)
		if err != nil {
			log.Printf("Error on search: %v\n", err)
			os.Exit(1)
		}
		for _, f := range res {
			fmt.Printf("\tName: %s\t\t Path: %s\n", f.Name, f.Path)
		}
		waitCh <- struct{}{}
	}()
	go func() {
		switch <-sigCh {
		case syscall.SIGUSR2:
			walker.IncreaseDepth(2)
		default:
			log.Println("Signal received, terminate...")
			cancel()
		}
	}()
	//Дополнительно: Ожидание всех горутин перед завершением
	<-waitCh
	log.Println("Done")
}
