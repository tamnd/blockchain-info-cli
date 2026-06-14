package blockchaininfo

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes blockchain.info as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/blockchain-info-cli/blockchain-info"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// blockchain-info:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone blockchain-info binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the blockchain-info driver.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "blockchaininfo",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "blockchain-info",
			Short:  "A command line for blockchain.info.",
			Long: `A command line for blockchain.info.

blockchain-info reads public blockchain.info data over plain HTTPS, shapes it into
clean records, and prints output that pipes into the rest of your tools. No API
key, nothing to run alongside it.`,
			Site: Host,
			Repo: "https://github.com/tamnd/blockchain-info-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// ticker: current BTC price in all available currencies.
	kit.Handle(app, kit.OpMeta{Name: "ticker", Group: "market", List: true,
		Summary: "List BTC price in all available currencies"}, getTicker)

	// stats: current Bitcoin network statistics.
	kit.Handle(app, kit.OpMeta{Name: "stats", Group: "network", Single: true,
		Summary: "Fetch Bitcoin network statistics"}, getStats)

	// block: details for a block by height.
	kit.Handle(app, kit.OpMeta{Name: "block", Group: "chain", Single: true,
		Summary:  "Fetch block details by height",
		URIType:  "height",
		Resolver: true,
		Args:     []kit.Arg{{Name: "height", Help: "block height"}}}, getBlock)

	// convert: convert a fiat amount to BTC.
	kit.Handle(app, kit.OpMeta{Name: "convert", Group: "market", Single: true,
		Summary:  "Convert a fiat amount to BTC",
		URIType:  "currency",
		Resolver: true,
		Args:     []kit.Arg{{Name: "currency", Help: "currency code e.g. USD"}}}, getConvert)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type tickerInput struct {
	Client *Client `kit:"inject"`
}

type statsInput struct {
	Client *Client `kit:"inject"`
}

type blockInput struct {
	Height int     `kit:"arg" help:"block height"`
	Client *Client `kit:"inject"`
}

type convertInput struct {
	Currency string  `kit:"arg" help:"currency code e.g. USD"`
	Value    float64 `kit:"flag" help:"amount to convert to BTC" default:"1"`
	Client   *Client `kit:"inject"`
}

// --- handlers ---

func getTicker(ctx context.Context, in tickerInput, emit func(*TickerPrice) error) error {
	body, err := in.Client.Get(ctx, BaseURL+"/ticker")
	if err != nil {
		return err
	}
	// Response is a JSON map: {"USD":{"last":N,"buy":N,"sell":N,"symbol":"$"},...}
	var raw map[string]struct {
		Last   float64 `json:"last"`
		Buy    float64 `json:"buy"`
		Sell   float64 `json:"sell"`
		Symbol string  `json:"symbol"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("ticker: decode: %w", err)
	}
	for currency, p := range raw {
		if err := emit(&TickerPrice{
			Currency: currency,
			Last:     p.Last,
			Buy:      p.Buy,
			Sell:     p.Sell,
			Symbol:   p.Symbol,
		}); err != nil {
			return err
		}
	}
	return nil
}

func getStats(ctx context.Context, in statsInput, emit func(*Stats) error) error {
	body, err := in.Client.Get(ctx, BaseURL+"/stats?format=json")
	if err != nil {
		return err
	}
	// Map the raw JSON fields to our Stats struct.
	var raw struct {
		MarketPriceUSD       float64 `json:"market_price_usd"`
		HashRate             float64 `json:"hash_rate"`
		NBlocksTotal         int64   `json:"n_blocks_total"`
		TotalBC              int64   `json:"totalbc"`
		NTx                  int64   `json:"n_tx"`
		Difficulty           float64 `json:"difficulty"`
		MinutesBetweenBlocks float64 `json:"minutes_between_blocks"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("stats: decode: %w", err)
	}
	return emit(&Stats{
		MarketPriceUSD:       raw.MarketPriceUSD,
		HashRate:             raw.HashRate,
		TotalBlocksMined:     raw.NBlocksTotal,
		TotalBTCMinted:       raw.TotalBC,
		TotalTransactions:    raw.NTx,
		Difficulty:           raw.Difficulty,
		MinutesBetweenBlocks: raw.MinutesBetweenBlocks,
	})
}

func getBlock(ctx context.Context, in blockInput, emit func(*Block) error) error {
	url := fmt.Sprintf("%s/block-height/%d?format=json", BaseURL, in.Height)
	body, err := in.Client.Get(ctx, url)
	if err != nil {
		return err
	}
	// Response is {"blocks":[{...}]}; we take blocks[0].
	var raw struct {
		Blocks []struct {
			Hash       string  `json:"hash"`
			Height     int     `json:"height"`
			Time       int64   `json:"time"`
			NTx        int     `json:"n_tx"`
			Size       int     `json:"size"`
			Difficulty float64 `json:"difficulty"`
			Nonce      int64   `json:"nonce"`
			Weight     int     `json:"weight"`
		} `json:"blocks"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("block: decode: %w", err)
	}
	if len(raw.Blocks) == 0 {
		return errs.NotFound("no block found at height %d", in.Height)
	}
	b := raw.Blocks[0]
	return emit(&Block{
		Hash:       b.Hash,
		Height:     b.Height,
		Time:       b.Time,
		TxCount:    b.NTx,
		Size:       b.Size,
		Difficulty: b.Difficulty,
		Nonce:      b.Nonce,
		Weight:     b.Weight,
	})
}

func getConvert(ctx context.Context, in convertInput, emit func(*Conversion) error) error {
	currency := strings.ToUpper(strings.TrimSpace(in.Currency))
	value := in.Value
	if value == 0 {
		value = 1
	}
	url := fmt.Sprintf("%s/tobtc?currency=%s&value=%g", BaseURL, currency, value)
	raw, err := in.Client.getRaw(ctx, url)
	if err != nil {
		return err
	}
	btc, err := strconv.ParseFloat(strings.TrimSpace(string(raw)), 64)
	if err != nil {
		return fmt.Errorf("convert: parse btc value %q: %w", string(raw), err)
	}
	return emit(&Conversion{
		Currency: currency,
		Value:    value,
		BTC:      btc,
	})
}

// --- Resolver: pure, network-free string functions ---

// Classify turns any accepted input into the canonical (type, id).
// Numeric string → ("height", input); 3-letter uppercase → ("currency", input).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty blockchain-info reference")
	}
	if isNumeric(input) {
		return "height", input, nil
	}
	if isCurrencyCode(input) {
		return "currency", strings.ToUpper(input), nil
	}
	// Default: treat as currency
	return "currency", strings.ToUpper(input), nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "height":
		return "https://www.blockchain.com/btc/block/" + id, nil
	case "currency":
		return "https://www.blockchain.com/explorer", nil
	default:
		return "", errs.Usage("blockchain-info has no resource type %q", uriType)
	}
}

// --- helpers ---

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func isCurrencyCode(s string) bool {
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return strings.ToUpper(s) == s
}

// mapErr converts a library error into the appropriate kit error kind.
func mapErr(err error) error {
	return err
}
