// Command perps demonstrates public reads, optional delegated account reads,
// and public streaming. It never creates credentials or submits trades.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/nijaru/go-clob-client/perps"
)

func main() {
	stream := flag.Bool(
		"stream",
		false,
		"stream public BBO updates until interrupted (maximum 30 seconds)",
	)
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := run(ctx, *stream); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run(ctx context.Context, streaming bool) error {
	config := perps.Config{Host: os.Getenv("PERPS_HOST"), WebSocketHost: os.Getenv("PERPS_WS_HOST")}
	client := perps.New(config)
	instruments, err := client.GetInstruments(ctx, perps.InstrumentsParams{})
	if err != nil {
		return fmt.Errorf("get instruments: %w", err)
	}
	if len(instruments) == 0 {
		fmt.Println("no instruments returned")
		return nil
	}
	inst := instruments[0]
	fmt.Printf(
		"instrument %d %s (category=%s, maxLeverage=%d, closeOnly=%t)\n",
		inst.ID,
		inst.Symbol,
		inst.Category,
		inst.MaxLeverage,
		inst.CloseOnly,
	)
	ticker, err := client.GetTicker(ctx, inst.ID)
	if err != nil {
		return fmt.Errorf("get ticker: %w", err)
	}
	fmt.Printf(
		"last=%s mark=%s index=%s fundingRate=%s\n",
		ticker.LastPrice,
		ticker.MarkPrice,
		ticker.IndexPrice,
		ticker.FundingRate,
	)
	book, err := client.GetBook(
		ctx,
		perps.BookParams{InstrumentID: inst.ID, Depth: perps.PerpsBookDepth100},
	)
	if err != nil {
		return fmt.Errorf("get book: %w", err)
	}
	fmt.Printf("book: %d bids, %d asks (seq=%d)\n", len(book.Bids), len(book.Asks), book.Sequence)
	fmt.Println("recent candles (1h):")
	for page, err := range client.IterCandles(ctx, perps.CandlesParams{InstrumentID: inst.ID, Interval: perps.PerpsKline1h}) {
		if err != nil {
			return fmt.Errorf("iter candles: %w", err)
		}
		for _, c := range page {
			fmt.Printf(
				"  %d open=%s high=%s low=%s close=%s vol=%s\n",
				c.Timestamp,
				c.Open,
				c.High,
				c.Low,
				c.Close,
				c.Volume,
			)
		}
	}
	// A private key is unnecessary for account reads. Do not log credentials.
	proxy, secret := os.Getenv("PERPS_PROXY"), os.Getenv("PERPS_SECRET")
	if proxy != "" || secret != "" {
		account, err := perps.NewAuthenticated(
			perps.AuthenticatedConfig{
				Config:      config,
				Credentials: perps.PerpsCredentials{Proxy: proxy, Secret: secret},
			},
		)
		if err != nil {
			return err
		}
		balances, err := account.GetBalances(ctx)
		if err != nil {
			return fmt.Errorf("get balances: %w", err)
		}
		fmt.Printf("account: %d collateral balance entries\n", len(balances))
	}
	if !streaming {
		return nil
	}
	updates, err := client.SubscribeMarket(
		ctx,
		[]perps.MarketSubscription{{Topic: perps.MarketBBO, InstrumentID: &inst.ID}},
	)
	if err != nil {
		return err
	}
	defer updates.Close()
	errors := updates.Errors()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-errors:
			if !ok {
				errors = nil
				continue
			}
			log.Printf("stream: %v", err)
		case event, ok := <-updates.Events():
			if !ok {
				return nil
			}
			if event.Resync != nil {
				fmt.Printf("resync: %s\n", event.Resync.Reason)
				continue
			}
			if event.Market != nil && event.Market.BBO != nil {
				bbo := event.Market.BBO
				fmt.Printf(
					"BBO: bid=%s/%s ask=%s/%s\n",
					bbo.BidPrice,
					bbo.BidQuantity,
					bbo.AskPrice,
					bbo.AskQuantity,
				)
			}
		}
	}
}
