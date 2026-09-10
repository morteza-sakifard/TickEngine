package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	// tzdata embeds the IANA timezone database so --tz keeps working on
	// machines without a system zoneinfo, which includes Windows once the
	// binary leaves this GOROOT.
	_ "time/tzdata"

	"github.com/morteza-sakifard/TickEngine/internal/core"
)

func main() {
	log.SetFlags(0)

	var (
		data   = flag.String("data", "", "path to the Databento CSV file (required)")
		schema = flag.String("schema", "", "mbp-1, mbp-10, or mbo; empty detects from the header")
		limit  = flag.Int64("limit", 0, "stop after N records; 0 means no limit")
		tick   = flag.String("tick", "0.25", "instrument tick size as a decimal")
		tzArg  = flag.String("tz", "America/Chicago", "timezone used to group records by calendar day")
	)
	flag.Parse()

	if *data == "" {
		flag.Usage()
		log.Fatal("--data is required")
	}

	tickNano, err := parsePriceNano(*tick)
	if err != nil {
		log.Fatalf("--tick %q: %v", *tick, err)
	}
	if tickNano <= 0 {
		log.Fatalf("--tick must be positive, got %q", *tick)
	}

	loc, err := time.LoadLocation(*tzArg)
	if err != nil {
		log.Fatalf("--tz %q: %v", *tzArg, err)
	}

	f, err := os.Open(*data)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	start := time.Now()
	c, err := Run(f, Options{TickNano: tickNano, Loc: loc, Limit: *limit, Schema: *schema})
	if err != nil {
		log.Fatal(err)
	}
	elapsed := time.Since(start)

	c.Report(os.Stdout)

	agg, err := runAggressorCheck(*data, core.ESZ5(), *limit, c.Schema)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println()
	reportAggressor(os.Stdout, agg)

	rate := 0.0
	if s := elapsed.Seconds(); s > 0 {
		rate = float64(c.Records) / s
	}
	fmt.Printf("\nscanned %d records in %s (%.0f records/sec)\n",
		c.Records, elapsed.Round(time.Millisecond), rate)
}
