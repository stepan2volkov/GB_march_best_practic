package main

//Исходники задания для первого занятия у других групп https://github.com/t0pep0/GB_best_go

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
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
	maxDepth               int64
	needPrintCurrentStatus <-chan struct{}
	wg                     *sync.WaitGroup
}

//Ограничить глубину поиска заданым числом, по SIGUSR2 увеличить глубину поиска на +2
func (w *DirectoryWalker) ListDirectory(ctx context.Context, ret chan<- FileInfo, dir string, depth int64) {
	defer w.wg.Done()
	if depth >= w.maxDepth {
		return
	}

	res, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("[ERROR] %v\n", err)
	}

	for _, entry := range res {
		time.Sleep(time.Second)
		select {
		case <-ctx.Done():
			return
		case <-w.needPrintCurrentStatus:
			fmt.Printf("\tDepth %d from %d.\n\tCWD: %s\n", depth, w.maxDepth, dir)
		default:
			path := filepath.Join(dir, entry.Name())
			log.Printf("[INFO] Scanning '%s'\n", path)
			if entry.IsDir() {
				w.wg.Add(1)
				go w.ListDirectory(ctx, ret, path, depth+1)
			} else {
				info, err := entry.Info()
				if err != nil {
					log.Printf("[ERROR] %v\n", err)
					continue
				}
				ret <- fileInfo{info, path}
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

	w.wg.Add(1)
	go w.ListDirectory(ctx, files, wd, 0)

	go func() {
		w.wg.Wait()
		close(files)
	}()
	var ret FileList
	for file := range files {
		if filepath.Ext(file.Name()) == ext {
			ret = append(ret, TargetFile{Name: file.Name(), Path: file.Path()})
		}
	}
	return ret, nil
}

func (w *DirectoryWalker) IncreaseDepth(delta int) {
	atomic.AddInt64(&w.maxDepth, int64(delta))
}

func main() {
	fmt.Println("Process ID:", os.Getpid())
	const wantExt = ".go"

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)

	needPrintCurrentStatus := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}

	walker := DirectoryWalker{maxDepth: 5, needPrintCurrentStatus: needPrintCurrentStatus, wg: wg}

	//Обработать сигнал SIGUSR1
	waitCh := make(chan struct{})
	go func() {
		res, err := walker.FindFiles(ctx, wantExt)
		if err != nil {
			log.Printf("[ERROR] Error on search: %v\n", err)
			os.Exit(1)
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
				log.Println("Signal received, terminate...")
				cancel()
				return
			}
		}
	}()
	//Дополнительно: Ожидание всех горутин перед завершением
	<-waitCh
	log.Println("Done")
}
