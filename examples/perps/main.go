// Command perps demonstrates the public Polymarket Perps market-data API.
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/nijaru/go-clob-client/perps"
)

func main() {
	ctx := context.Background()
	client := perps.New(perps.Config{})

	instruments, err := client.GetInstruments(ctx, perps.InstrumentsParams{})
	if err != nil {
		log.Fatalf("get instruments: %v", err)
	}
	if len(instruments) == 0 {
		fmt.Println("no instruments returned")
		os.Exit(0)
	}

	inst := instruments[0]
	fmt.Printf("instrument %d %s (category=%s, maxLeverage=%d)\n",
		inst.ID, inst.Symbol, inst.Category, inst.MaxLeverage)

	ticker, err := client.GetTicker(ctx, inst.ID)
	if err != nil {
		log.Fatalf("get ticker: %v", err)
	}
	fmt.Printf("last=%s mark=%s index=%s fundingRate=%s\n",
		ticker.LastPrice, ticker.MarkPrice, ticker.IndexPrice, ticker.FundingRate)

	book, err := client.GetBook(ctx, perps.BookParams{InstrumentID: inst.ID, Depth: perps.PerpsBookDepth100})
	if err != nil {
		log.Fatalf("get book: %v", err)
	}
	fmt.Printf("book: %d bids, %d asks (seq=%d)\n", len(book.Bids), len(book.Asks), book.Sequence)

	fmt.Println("recent candles (1h):")
	for page, err := range client.IterCandles(ctx, perps.CandlesParams{
		InstrumentID: inst.ID,
		Interval:     perps.PerpsKline1h,
	}) {
		if err != nil {
			log.Fatalf("iter candles: %v", err)
		}
		for _, c := range page {
			fmt.Printf("  %d open=%s high=%s low=%s close=%s vol=%s\n",
				c.Timestamp, c.Open, c.High, c.Low, c.Close, c.Volume)
		}
	}
}
