package main

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/Room-Elephant/pluck/internal/client"
	"github.com/Room-Elephant/pluck/internal/client/transmission"
	"github.com/Room-Elephant/pluck/internal/config"
	"github.com/Room-Elephant/pluck/internal/log"
	"github.com/Room-Elephant/pluck/internal/placer"
	"github.com/Room-Elephant/pluck/internal/rules"
	"github.com/Room-Elephant/pluck/internal/watcher"
)

func main() {
	appConfig, err := config.Load()
	if err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}
	log.SetLevel(appConfig.LogLevel)

	log.Infof("pluck starting…")
	log.Infof("client: %s | mode: %s | watch: %s", appConfig.Client, appConfig.Mode, appConfig.WatchDir)

	if appConfig.DryRun {
		log.Infof("dry-run mode enabled — no files will be moved")
	}

	ruleset, err := rules.Load(appConfig.RulesFile)
	if err != nil {
		log.Errorf("%v", err)
		os.Exit(1)
	}

	torrentClient := newClient(appConfig)

	filePlacer := placer.New(appConfig.Mode)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := torrentClient.WaitForReady(ctx); err != nil {
		log.Errorf("waiting for client: %v", err)
		os.Exit(1)
	}
	log.Infof("client is ready — running initial scan…")

	var scanMu sync.Mutex
	scan := func() {
		scanMu.Lock()
		defer scanMu.Unlock()
		pluckTorrents(ctx, torrentClient, ruleset, filePlacer, appConfig)
	}

	watchErr := make(chan error, 1)
	go func() {
		watchErr <- watcher.Watch(ctx, appConfig.WatchDir, appConfig.RescanInterval, scan)
	}()

	scan()

	if err := <-watchErr; err != nil {
		if ctx.Err() == nil {
			log.Errorf("watcher exited: %v", err)
			os.Exit(1)
		}
	}

	log.Infof("pluck stopped.")
}

func newClient(appConfig config.Config) client.Client {
	switch appConfig.Client {
	case "transmission":
		return transmission.New(appConfig.ClientURL, appConfig.ClientUser, appConfig.ClientPass)
	}
	panic("unreachable: unsupported client " + appConfig.Client)
}
