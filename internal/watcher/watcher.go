package watcher

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Room-Elephant/pluck/internal/log"
)

const (
	debounce = 2 * time.Second
)

// ScanFunc is called by the watcher whenever a scan should be performed
// (either from a filesystem event or a periodic rescan tick).
type ScanFunc func()

// Watch starts two concurrent mechanisms that trigger fn:
//
//  1. A recursive fsnotify watcher on watchDir (create / rename events),
//     debounced by 2 s to let files settle before scanning.
//  2. A periodic ticker that fires every rescanInterval.
//
// Watch blocks until ctx is cancelled.
func Watch(ctx context.Context, watchDir string, rescanInterval time.Duration, scanCallback ScanFunc) error {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsWatcher.Close()

	// Register the root and all existing subdirectories.
	if err := registerAll(fsWatcher, watchDir); err != nil {
		return err
	}

	ticker := time.NewTicker(rescanInterval)
	defer ticker.Stop()

	// debounceTimer fires debounce after the last filesystem event.
	// Starts in a stopped state; Reset is called on each event.
	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
	}

	log.Infof("watching %s for changes…", watchDir)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		// --- Periodic rescan ---
		case <-ticker.C:
			log.Debugf("periodic rescan triggered")
			scanCallback()

		// --- Filesystem events ---
		case event, ok := <-fsWatcher.Events:
			if !ok {
				return nil
			}

			// If a new directory appeared, register it so we catch files
			// created inside it. This replicates `inotifywait -r` depth.
			if event.Has(fsnotify.Create) {
				if fileInfo, err := os.Stat(event.Name); err == nil && fileInfo.IsDir() {
					if err := registerAll(fsWatcher, event.Name); err != nil {
						log.Debugf("could not watch new dir %s: %v", event.Name, err)
					}
				}
			}

			// Only react to create / rename-into events, mirroring
			// inotifywait's `-e create -e moved_to` flags.
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
				log.Debugf("fs event: %s — debouncing %s", event, debounce)
				// Reset the debounce timer: discard any pending tick first.
				if !debounceTimer.Stop() {
					select {
					case <-debounceTimer.C:
					default:
					}
				}
				debounceTimer.Reset(debounce)
			}

		case <-debounceTimer.C:
			scanCallback()

		case err, ok := <-fsWatcher.Errors:
			if !ok {
				return nil
			}
			log.Errorf("watcher error: %v", err)
		}
	}
}

// registerAll adds watchDir and every subdirectory beneath it to the watcher.
// New subdirectories discovered during a watch run are added by the event
// loop above, closing the recursive-watch gap that WatchService doesn't handle.
func registerAll(fsWatcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			log.Debugf("walk error at %s: %v", path, err)
			return nil
		}
		if fileInfo.IsDir() {
			if watchErr := fsWatcher.Add(path); watchErr != nil {
				log.Debugf("could not watch %s: %v", path, watchErr)
			}
		}
		return nil
	})
}
