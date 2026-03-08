package transmission

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Room-Elephant/pluck/internal/client"
	"github.com/Room-Elephant/pluck/internal/log"
)

const (
	sessionIDHeader       = "X-Transmission-Session-Id"
	statusSessionRequired = 409
)

type Client struct {
	url       string
	user      string
	pass      string
	http      *http.Client
	sessionID string
}

func New(url, user, pass string) *Client {
	return &Client{
		url:  url,
		user: user,
		pass: pass,
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

// ---------------------------------------------------------------------------
// client.Client interface
// ---------------------------------------------------------------------------
func (transmissionClient *Client) WaitForReady(ctx context.Context) error {
	log.Infof("waiting for Transmission at %s…", transmissionClient.url)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fetchedSessionID, err := transmissionClient.fetchSessionID(ctx)
		if err == nil && fetchedSessionID != "" {
			transmissionClient.sessionID = fetchedSessionID
			return nil
		}

		log.Debugf("transmission not ready yet, retrying in 5 s…")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
		}
	}
}

func (transmissionClient *Client) CompletedTorrents(ctx context.Context) ([]client.Torrent, error) {
	responseBody, err := transmissionClient.rpc(ctx, "torrent-get", map[string]any{
		"fields": []string{"name", "downloadDir", "labels", "percentDone"},
	})
	if err != nil {
		return nil, fmt.Errorf("torrent-get RPC: %w", err)
	}

	var rpcResponseData rpcResponse
	if err := json.Unmarshal(responseBody, &rpcResponseData); err != nil {
		return nil, fmt.Errorf("parsing torrent-get response: %w", err)
	}

	var torrents []client.Torrent
	for _, rpcTorrentItem := range rpcResponseData.Arguments.Torrents {
		if rpcTorrentItem.PercentDone < 1.0 {
			continue
		}
		for _, label := range rpcTorrentItem.Labels {
			torrents = append(torrents, client.Torrent{
				Label: strings.ToLower(label),
				Path:  path.Join(rpcTorrentItem.DownloadDir, rpcTorrentItem.Name),
			})
		}
	}
	return torrents, nil
}

// ---------------------------------------------------------------------------
// RPC machinery
// ---------------------------------------------------------------------------
func (transmissionClient *Client) rpc(ctx context.Context, method string, args any) ([]byte, error) {
	payload, err := json.Marshal(rpcRequest{Method: method, Arguments: args})
	if err != nil {
		return nil, err
	}

	responseBody, retry, err := transmissionClient.doRequest(ctx, payload)
	if err != nil {
		return nil, err
	}
	if retry {
		responseBody, _, err = transmissionClient.doRequest(ctx, payload)
		if err != nil {
			return nil, err
		}
	}
	return responseBody, nil
}

func (transmissionClient *Client) doRequest(ctx context.Context, payload []byte) (responseBody []byte, retry bool, err error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, transmissionClient.url, bytes.NewReader(payload))
	if err != nil {
		return nil, false, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set(sessionIDHeader, transmissionClient.sessionID)
	if transmissionClient.user != "" {
		httpRequest.SetBasicAuth(transmissionClient.user, transmissionClient.pass)
	}

	httpResponse, err := transmissionClient.http.Do(httpRequest)
	if err != nil {
		return nil, false, fmt.Errorf("HTTP request to Transmission failed: %w", err)
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode == statusSessionRequired {
		transmissionClient.sessionID = httpResponse.Header.Get(sessionIDHeader)
		log.Debugf("refreshed Transmission session ID")
		return nil, true, nil
	}

	if httpResponse.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("unexpected HTTP status %d from Transmission", httpResponse.StatusCode)
	}

	responseData, err := io.ReadAll(httpResponse.Body)
	return responseData, false, err
}

func (transmissionClient *Client) fetchSessionID(ctx context.Context) (string, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, transmissionClient.url, nil)
	if err != nil {
		return "", err
	}
	if transmissionClient.user != "" {
		httpRequest.SetBasicAuth(transmissionClient.user, transmissionClient.pass)
	}

	httpResponse, err := transmissionClient.http.Do(httpRequest)
	if err != nil {
		return "", err
	}
	httpResponse.Body.Close()

	if httpResponse.StatusCode == statusSessionRequired {
		return httpResponse.Header.Get(sessionIDHeader), nil
	}
	return "", fmt.Errorf("unexpected status %d", httpResponse.StatusCode)
}

// ---------------------------------------------------------------------------
// JSON shapes
// ---------------------------------------------------------------------------

type rpcRequest struct {
	Method    string `json:"method"`
	Arguments any    `json:"arguments"`
}

type rpcResponse struct {
	Arguments struct {
		Torrents []rpcTorrent `json:"torrents"`
	} `json:"arguments"`
}

type rpcTorrent struct {
	Name        string   `json:"name"`
	DownloadDir string   `json:"downloadDir"`
	Labels      []string `json:"labels"`
	PercentDone float64  `json:"percentDone"`
}
