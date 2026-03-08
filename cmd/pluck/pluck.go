package main

import (
	"context"
	"os"

	"github.com/Room-Elephant/pluck/internal/client"
	"github.com/Room-Elephant/pluck/internal/config"
	"github.com/Room-Elephant/pluck/internal/log"
	"github.com/Room-Elephant/pluck/internal/placer"
	"github.com/Room-Elephant/pluck/internal/rules"
	"github.com/Room-Elephant/pluck/internal/state"
)

func pluckTorrents(ctx context.Context, torrentClient client.Client, activeRuleset rules.Ruleset, filePlacer *placer.Placer, appState *state.State, appConfig config.Config) {
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

		if appState.HasProcessed(torrent.Path, torrent.Label) {
			log.Debugf("already processed, skipping: %s (%s)", name, torrent.Label)
			continue
		}

		if exists(dst) {
			log.Debugf("already exists, skipping: %s", name)
			// Mark as processed so we don't check filesystem on future runs
			if !appConfig.DryRun {
				if err := appState.MarkProcessed(torrent.Path, torrent.Label); err != nil {
					log.Errorf("failed to save state for existing file %s: %v", name, err)
				}
			}
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
			if err := appState.MarkProcessed(torrent.Path, torrent.Label); err != nil {
				log.Errorf("failed to save state for %s: %v", name, err)
			}
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
