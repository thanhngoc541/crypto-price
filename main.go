package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// APIResponse represents the structure of the API response
type APIResponse struct {
	Source string `json:"source"`
	Price  string `json:"price"`
}

// Map of CoinGecko identifiers for different symbols
var coinGeckoSymbols = map[string]string{
	"BTC":  "bitcoin",
	"ETH":  "ethereum",
	"SOL":  "solana",
	"DOGE": "dogecoin",
	"SHIB": "shiba-inu",
}

// Map of Kraken identifiers for different symbols
var krakenSymbols = map[string]string{
	"BTC":  "XXBTZUSD",
	"ETH":  "XETHZUSD",
	"SOL":  "SOLUSD",
	"DOGE": "XDGUSD",
	"SHIB": "SHIBUSD",
}

// Utility function to make an HTTP GET request and decode the JSON response
func fetchPrice(url string, target interface{}) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error fetching price, status code: %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}

	return nil
}

// Fetch price from Binance
func getPriceFromBinance(symbol string) (string, error) {
	url := fmt.Sprintf("https://api.binance.com/api/v3/ticker/price?symbol=%sUSDT", strings.ToUpper(symbol))
	var result struct {
		Price string `json:"price"`
	}
	err := fetchPrice(url, &result)
	if err != nil {
		log.Printf("Binance error: %v", err)
	}
	return result.Price, err
}

// Fetch price from CoinGecko
func getPriceFromCoinGecko(symbol string) (string, error) {
	coinGeckoSymbol, ok := coinGeckoSymbols[strings.ToUpper(symbol)]
	if !ok {
		return "", fmt.Errorf("unknown symbol for CoinGecko: %s", symbol)
	}
	url := fmt.Sprintf("https://api.coingecko.com/api/v3/simple/price?ids=%s&vs_currencies=usd", coinGeckoSymbol)
	var result map[string]map[string]float64
	err := fetchPrice(url, &result)
	if err != nil {
		log.Printf("CoinGecko error: %v", err)
		return "", err
	}
	price := result[coinGeckoSymbol]["usd"]
	return fmt.Sprintf("%.2f", price), nil
}

// Fetch price from Kraken
func getPriceFromKraken(symbol string) (string, error) {
	krakenPair, ok := krakenSymbols[strings.ToUpper(symbol)]
	if !ok {
		return "", fmt.Errorf("unknown symbol for Kraken: %s", symbol)
	}

	url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", krakenPair)
	var result struct {
		Result map[string]struct {
			C []string `json:"c"`
		} `json:"result"`
	}
	err := fetchPrice(url, &result)
	if err != nil {
		log.Printf("Kraken error: %v", err)
		return "", err
	}
	priceList := result.Result[krakenPair]
	if len(priceList.C) == 0 {
		return "", fmt.Errorf("price not found for %s", symbol)
	}
	return priceList.C[0], nil
}

// Fetch price from Coinbase
func getPriceFromCoinbase(symbol string) (string, error) {
	url := fmt.Sprintf("https://api.coinbase.com/v2/prices/%s-USD/spot", strings.ToUpper(symbol))
	var result struct {
		Data struct {
			Amount string `json:"amount"`
		} `json:"data"`
	}
	err := fetchPrice(url, &result)
	if err != nil {
		log.Printf("Coinbase error: %v", err)
	}
	return result.Data.Amount, err
}

// Fetch prices from all sources concurrently
func fetchPricesConcurrently(symbol string) []APIResponse {
	var wg sync.WaitGroup
	var mu sync.Mutex
	sources := []struct {
		Name   string
		Fetch  func(string) (string, error)
		Symbol string
	}{
		{"Binance", getPriceFromBinance, symbol},
		{"CoinGecko", getPriceFromCoinGecko, symbol},
		{"Kraken", getPriceFromKraken, symbol},
		{"Coinbase", getPriceFromCoinbase, symbol},
	}

	prices := make([]APIResponse, len(sources))
	for i, source := range sources {
		wg.Add(1)
		go func(i int, source struct {
			Name   string
			Fetch  func(string) (string, error)
			Symbol string
		}) {
			defer wg.Done()
			price, err := source.Fetch(source.Symbol)
			if err != nil {
				log.Printf("Error fetching price from %s for symbol %s: %v", source.Name, source.Symbol, err)
				price = "Error fetching price"
			}
			mu.Lock()
			prices[i] = APIResponse{Source: fmt.Sprintf("%s (%s)", source.Name, strings.ToUpper(symbol)), Price: price}
			mu.Unlock()
		}(i, source)
	}

	wg.Wait()
	return prices
}

func main() {
	r := gin.Default()

	// Dynamic route to fetch the price based on the symbol
	r.GET("/price/:symbol", func(c *gin.Context) {
		symbol := c.Param("symbol")
		prices := fetchPricesConcurrently(symbol)
		c.JSON(http.StatusOK, gin.H{"prices": prices})
	})

	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}
}
