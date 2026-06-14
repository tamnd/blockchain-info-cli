package blockchaininfo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	blockchaininfo "github.com/tamnd/blockchain-info-cli/blockchain-info"
)

func TestGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := blockchaininfo.NewClient()
	c.Rate = 0 // no pacing in the test

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := blockchaininfo.NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestGetNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := blockchaininfo.NewClient()
	c.Rate = 0
	c.Retries = 0

	_, err := c.Get(context.Background(), srv.URL)
	if err == nil {
		t.Error("expected error for 404, got nil")
	}
}

func TestTickerTypes(t *testing.T) {
	// Verify TickerPrice struct fields are accessible and typed correctly.
	p := blockchaininfo.TickerPrice{
		Currency: "USD",
		Last:     64067.96,
		Buy:      64100,
		Sell:     64035,
		Symbol:   "$",
	}
	if p.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", p.Currency)
	}
	if p.Last != 64067.96 {
		t.Errorf("Last = %v, want 64067.96", p.Last)
	}
}

func TestStatsTypes(t *testing.T) {
	s := blockchaininfo.Stats{
		MarketPriceUSD:       64021.47,
		HashRate:             751464021029.33,
		TotalBlocksMined:     953653,
		TotalBTCMinted:       2004266562500000,
		TotalTransactions:    562493,
		Difficulty:           90666502495030.7,
		MinutesBetweenBlocks: 8.32,
	}
	if s.TotalBlocksMined != 953653 {
		t.Errorf("TotalBlocksMined = %d, want 953653", s.TotalBlocksMined)
	}
	if s.TotalBTCMinted != 2004266562500000 {
		t.Errorf("TotalBTCMinted = %d, want 2004266562500000", s.TotalBTCMinted)
	}
}

func TestBlockTypes(t *testing.T) {
	b := blockchaininfo.Block{
		Hash:       "000000000000000000abc123",
		Height:     853652,
		Time:       1721786732,
		TxCount:    2276,
		Size:       1249327,
		Difficulty: 90666502495030.0,
		Nonce:      2654735100,
		Weight:     3993180,
	}
	if b.Hash != "000000000000000000abc123" {
		t.Errorf("Hash = %q", b.Hash)
	}
	if b.TxCount != 2276 {
		t.Errorf("TxCount = %d, want 2276", b.TxCount)
	}
}

func TestConversionTypes(t *testing.T) {
	c := blockchaininfo.Conversion{
		Currency: "USD",
		Value:    100,
		BTC:      0.00156084,
	}
	if c.Currency != "USD" {
		t.Errorf("Currency = %q, want USD", c.Currency)
	}
	if c.BTC != 0.00156084 {
		t.Errorf("BTC = %v, want 0.00156084", c.BTC)
	}
}

func TestNewClientDefaults(t *testing.T) {
	c := blockchaininfo.NewClient()
	if c.Rate != 500*time.Millisecond {
		t.Errorf("Rate = %v, want 500ms", c.Rate)
	}
	if c.Retries != 5 {
		t.Errorf("Retries = %d, want 5", c.Retries)
	}
	if c.HTTP == nil {
		t.Error("HTTP client is nil")
	}
	if c.UserAgent == "" {
		t.Error("UserAgent is empty")
	}
}

func TestTickerJSONRoundtrip(t *testing.T) {
	// Simulate the ticker endpoint: returns a map of currency -> price object.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"USD": map[string]any{"last": 64067.96, "buy": 64100.0, "sell": 64035.0, "symbol": "$"},
			"EUR": map[string]any{"last": 55394.62, "buy": 55420.0, "sell": 55369.0, "symbol": "€"},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := blockchaininfo.NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]struct {
		Last   float64 `json:"last"`
		Symbol string  `json:"symbol"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(raw) != 2 {
		t.Errorf("got %d currencies, want 2", len(raw))
	}
	usd, ok := raw["USD"]
	if !ok {
		t.Fatal("USD not in response")
	}
	if usd.Last != 64067.96 {
		t.Errorf("USD.Last = %v, want 64067.96", usd.Last)
	}
}

func TestGetPlainTextFloat(t *testing.T) {
	// Simulate /tobtc returning a plain float string.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "0.00156084")
	}))
	defer srv.Close()

	c := blockchaininfo.NewClient()
	c.Rate = 0

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "0.00156084" {
		t.Errorf("body = %q, want 0.00156084", string(body))
	}
}
