package main

import (
	"context"
	"os"

	"github.com/Room-Elephant/pluck/internal/client"
	"github.com/Room-Elephant/pluck/internal/config"
	"github.com/Room-Elephant/pluck/internal/log"
	"github.com/Room-Elephant/pluck/internal/placer"
	"github.com/Room-Elephant/pluck/internal/rules"
)

func pluckTorrents(ctx context.Context, torrentClient client.Client, activeRuleset rules.Ruleset, filePlacer *placer.Placer, appConfig config.Config) {
	log.Debugf("scanning for completed torrents…")

	torrents, err := torrentClient.CompletedTorrents(ctx)
	if err != nil {
		log.Debugf("could not retrieve torrents: %v", err)
		return
	}

	for _, torrent := range torrents {
		destDir := activeRuleset.Destination(torrent.Label)
		if destDir == "" {
			log.Debugf("no rule for label %q, skipping: %s", torrent.Label, torrent.Path)
			continue
		}

		name := torrent.Path[lastSlash(torrent.Path)+1:]
		dst := destDir + "/" + name

		if exists(dst) {
			log.Debugf("already exists, skipping: %s", name)
			continue
		}

		if !exists(torrent.Path) {
			log.Debugf("source not found, skipping: %s", torrent.Path)
			continue
		}

		if appConfig.DryRun {
			log.Infof("[dry-run] would %s: %s -> %s", filePlacer.Mode(), name, destDir)
			continue
		}

		if _, err := filePlacer.Place(torrent.Path, destDir); err != nil {
			log.Errorf("failed to pluck %s -> %s: %v", name, destDir, err)
		} else {
			log.Infof("plucked (%s): %s -> %s", filePlacer.Mode(), name, destDir)
		}
	}
}

func lastSlash(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' {
			return i
		}
	}
	return -1
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
