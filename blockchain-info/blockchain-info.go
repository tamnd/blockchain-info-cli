// Package blockchaininfo is the library behind the blockchain-info command line:
// the HTTP client, request shaping, and the typed data models for blockchain.info.
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public site throws under load.
package blockchaininfo

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultUserAgent identifies the client to blockchain.info.
const DefaultUserAgent = "blockchain-info-cli/dev (+https://github.com/tamnd/blockchain-info-cli)"

// Host is the site this client talks to.
const Host = "blockchain.info"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Client talks to blockchain.info over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults: a 30s timeout, a 500ms
// minimum gap between requests, and five retries on transient errors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   5,
	}
}

// Get fetches url and returns the response body. It paces and retries according
// to the client's settings. The caller owns nothing extra; the body is read
// fully and closed here.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	return c.getRaw(ctx, url)
}

// getRaw fetches url and returns the raw response bytes. This is used for
// endpoints that return plain text (not JSON), like /tobtc and /q/getblockcount.
func (c *Client) getRaw(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// TickerPrice holds the current price for one currency from /ticker.
type TickerPrice struct {
	Currency string  `kit:"id" json:"currency"`
	Last     float64 `json:"last"`
	Buy      float64 `json:"buy"`
	Sell     float64 `json:"sell"`
	Symbol   string  `json:"symbol"`
}

// Stats holds network statistics from /stats?format=json.
type Stats struct {
	MarketPriceUSD       float64 `kit:"id" json:"market_price_usd"`
	HashRate             float64 `json:"hash_rate"`
	TotalBlocksMined     int64   `json:"total_blocks_mined"`
	TotalBTCMinted       int64   `json:"total_btc_minted_satoshi"`
	TotalTransactions    int64   `json:"total_transactions"`
	Difficulty           float64 `json:"difficulty"`
	MinutesBetweenBlocks float64 `json:"minutes_between_blocks"`
}

// Block holds summary data for one block from /block-height/{height}?format=json.
type Block struct {
	Hash       string  `kit:"id" json:"hash"`
	Height     int     `json:"height"`
	Time       int64   `json:"time"`
	TxCount    int     `json:"tx_count"`
	Size       int     `json:"size"`
	Difficulty float64 `json:"difficulty"`
	Nonce      int64   `json:"nonce"`
	Weight     int     `json:"weight"`
}

// Conversion holds the result of converting a fiat amount to BTC.
type Conversion struct {
	Currency string  `kit:"id" json:"currency"`
	Value    float64 `json:"fiat_value"`
	BTC      float64 `json:"btc"`
}
