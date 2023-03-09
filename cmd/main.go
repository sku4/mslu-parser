package main

import (
	"context"
	"flag"
	"github.com/sku4/mslu-parser/models/cli"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/lib/pq"
	"github.com/sku4/mslu-parser/internal/repository"
	"github.com/sku4/mslu-parser/internal/service"
	"github.com/sku4/mslu-parser/pkg/logger"
)

func main() {
	// init context
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	args := cli.Arguments{}
	flag.IntVar(&args.Count, "count", 100, "The count param")
	flag.StringVar(&args.Profile, "profile", "", "The profile param: zeit")
	flag.BoolVar(&args.Update, "update", false, "Update param")
	flag.Parse()
	ctx = cli.SetArgs(ctx, args)

	repos := repository.NewRepository()
	services := service.NewService(ctx, repos)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	_ = services

	go func() {
		if err := services.Parser.Run(ctx); err != nil {
			logger.Info(err.Error())
			quit <- nil
		}
	}()

	logger.Infos("App Started with args", args)

	// graceful shutdown
	logger.Infof("Got signal %v, attempting graceful shutdown", <-quit)
	cancel()
	logger.Info("Context is stopped")

	err := services.Parser.Shutdown()
	if err != nil {
		logger.Infof("error parser shutdown: %s", err.Error())
	} else {
		logger.Info("Parser stopped")
	}

	logger.Info("App Shutting Down")
}
