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
	flag.IntVar(&args.Count, "count", 100, "Count download articles")
	flag.StringVar(&args.Profile, "profile", "", "Available: zeit")
	flag.StringVar(&args.Login, "login", "", "Auth login")
	flag.StringVar(&args.Password, "pass", "", "Auth password")
	flag.BoolVar(&args.Update, "update", false, "Update downloaded articles")
	flag.Parse()
	ctx = cli.SetArgs(ctx, args)

	repos := repository.NewRepository()
	services := service.NewService(repos)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	log := logger.Get()

	go func() {
		if err := services.Parser.Run(ctx); err != nil {
			log.Info(err.Error())
		}
		quit <- nil
	}()

	log.Infof("App Started with args: '%s', count %d, update %t", args.Profile, args.Count, args.Update)

	// graceful shutdown
	log.Infof("Got signal %v, attempting graceful shutdown", <-quit)
	cancel()
	log.Info("Context is stopped")

	err := services.Parser.Shutdown()
	if err != nil {
		log.Infof("error parser shutdown: %s", err.Error())
	} else {
		log.Info("Parser stopped")
	}

	log.Info("App Shutting Down")
}