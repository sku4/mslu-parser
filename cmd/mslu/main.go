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
	flag.StringVar(&args.Profile, "profile", "", "Available: zeit, spiegel")
	flag.StringVar(&args.Login, "login", "", "Auth login")
	flag.StringVar(&args.Password, "pass", "", "Auth password")
	flag.BoolVar(&args.Update, "update", false, "Update downloaded articles")
	flag.StringVar(&args.ZeitMode, "zeit_mode", "1y", "Zeit mode")
	flag.StringVar(&args.ZeitType, "zeit_type", "article", "Zeit type")
	flag.IntVar(&args.SpiegelZeitraum, "spiegel_zeitraum", 365, "Spiegel zeitraum (in days)")
	flag.StringVar(&args.SpiegelSuchbegriff, "spiegel_suchbegriff", "politik", "Spiegel suchbegriff")
	flag.StringVar(&args.SpiegelInhalt, "spiegel_inhalt", "", "Spiegel inhalt (heading,title,intro)")
	flag.StringVar(&args.SpiegelSegments, "spiegel_segments",
		"spon,spon_paid,spon_international,mmo,mmo_paid,hbm,hbm_paid", "Spiegel segments")
	flag.Parse()
	ctx = cli.SetArgs(ctx, args)

	repos := repository.NewRepository()
	services := service.NewService(repos)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	log := logger.Get()
	log.Infof("App Started with args: '%s', count %d, update %t", args.Profile, args.Count, args.Update)

	go func() {
		if err := services.Parser.Run(ctx); err != nil {
			log.Info(err.Error())
		}
		quit <- nil
	}()

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
