package main

import (
	"context"
	"os"
	"os/signal"
	"practic/internal/walker"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	logger := initLogger()

	logger.Info("proccess ID", zap.Int("pid", os.Getpid()))
	const wantExt = ".go"

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)

	needPrintCurrentStatus := make(chan struct{}, 1)
	wg := &sync.WaitGroup{}

	app := walker.New(logger, 5, needPrintCurrentStatus, wg)

	waitCh := make(chan struct{})

	wd, err := os.Getwd()
	if err != nil {
		logger.Fatal("error when getting current working dir", zap.Error(err))
	}

	go func() {
		res, err := app.FindFiles(ctx, wd, wantExt)
		if err != nil {
			logger.Fatal("error on search", zap.Error(err))
		}
		for _, f := range res {
			logger.Info("File found",
				zap.String("filename", f.Name),
				zap.String("filepath", f.Path))
		}
		waitCh <- struct{}{}
	}()
	go func() {
		for {
			switch <-sigCh {
			case syscall.SIGUSR1:
				needPrintCurrentStatus <- struct{}{}
			case syscall.SIGUSR2:
				app.IncreaseDepth(2)
			default:
				logger.Warn("Signal received, terminate...")
				cancel()
				return
			}
		}
	}()

	<-waitCh
	logger.Info("Done")
}

func initLogger() *zap.Logger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.ISO8601TimeEncoder

	fileEncoder := zapcore.NewJSONEncoder(config)

	writer := zapcore.AddSync(os.Stdout)

	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, writer, zapcore.DebugLevel),
	)
	return zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}
