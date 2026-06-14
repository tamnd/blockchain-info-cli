package blockchaininfo

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring, which need no network. The client's HTTP behaviour is
// covered in blockchain-info_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "blockchaininfo" {
		t.Errorf("Scheme = %q, want blockchaininfo", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "blockchain-info" {
		t.Errorf("Identity.Binary = %q, want blockchain-info", info.Identity.Binary)
	}
}

func TestClassifyHeight(t *testing.T) {
	typ, id, err := Domain{}.Classify("853652")
	if err != nil {
		t.Fatalf("Classify height: %v", err)
	}
	if typ != "height" || id != "853652" {
		t.Errorf("Classify(853652) = (%q, %q), want (height, 853652)", typ, id)
	}
}

func TestClassifyCurrency(t *testing.T) {
	typ, id, err := Domain{}.Classify("USD")
	if err != nil {
		t.Fatalf("Classify currency: %v", err)
	}
	if typ != "currency" || id != "USD" {
		t.Errorf("Classify(USD) = (%q, %q), want (currency, USD)", typ, id)
	}
}

func TestClassifyLowercaseCurrency(t *testing.T) {
	// Non-3-letter-uppercase falls through to default currency treatment.
	typ, id, err := Domain{}.Classify("eur")
	if err != nil {
		t.Fatalf("Classify eur: %v", err)
	}
	if typ != "currency" {
		t.Errorf("Classify(eur) type = %q, want currency", typ)
	}
	_ = id
}

func TestLocateHeight(t *testing.T) {
	got, err := Domain{}.Locate("height", "853652")
	want := "https://www.blockchain.com/btc/block/853652"
	if err != nil || got != want {
		t.Errorf("Locate(height, 853652) = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateCurrency(t *testing.T) {
	got, err := Domain{}.Locate("currency", "USD")
	want := "https://www.blockchain.com/explorer"
	if err != nil || got != want {
		t.Errorf("Locate(currency, USD) = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("Locate(unknown, foo) expected error, got nil")
	}
}

func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	// Conversion has a Resolver op so it ends up in the mint index.
	c := &Conversion{Currency: "USD", Value: 100, BTC: 0.00156084}
	u, err := h.Mint(c)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "blockchaininfo://currency/USD"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("blockchaininfo", "USD")
	if err != nil || got.String() != "blockchaininfo://currency/USD" {
		t.Errorf("ResolveOn = (%q, %v), want blockchaininfo://currency/USD", got.String(), err)
	}
}

func TestIsNumeric(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"853652", true},
		{"0", true},
		{"", false},
		{"123abc", false},
		{"USD", false},
	}
	for _, tc := range cases {
		got := isNumeric(tc.in)
		if got != tc.want {
			t.Errorf("isNumeric(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestIsCurrencyCode(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"USD", true},
		{"EUR", true},
		{"GBP", true},
		{"usd", false},
		{"US", false},
		{"USDT", false},
		{"123", false},
	}
	for _, tc := range cases {
		got := isCurrencyCode(tc.in)
		if got != tc.want {
			t.Errorf("isCurrencyCode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
