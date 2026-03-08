package client

import "context"

// SupportedClients returns the list of torrent clients supported by pluck.
func SupportedClients() []string {
	return []string{"transmission"}
}

// Torrent represents a completed torrent with a single label applied.
// A torrent with multiple labels produces one Torrent value per label,
// matching the shell script's behaviour of emitting one line per label.
type Torrent struct {
	// Label is the lower-cased label attached to the torrent.
	Label string
	// Path is the full filesystem path to the downloaded content
	// (downloadDir + "/" + name).
	Path string
}

// Client is the interface that any torrent backend must satisfy.
// Adding a new client (qBittorrent, Deluge, …) means implementing
// this interface — no changes to core pluck logic are required.
type Client interface {
	// WaitForReady blocks until the torrent client is reachable and
	// accepting requests, or until ctx is cancelled.
	WaitForReady(ctx context.Context) error

	// CompletedTorrents returns every (label, path) pair for torrents
	// whose download is 100 % complete.
	CompletedTorrents(ctx context.Context) ([]Torrent, error)
}
