// Single-writer process: one binary owns both the HTTP server and the scraper.
// No external coordination is needed or supported.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"oreilly-cache/internal/scraper"
	"oreilly-cache/internal/server"
	"oreilly-cache/internal/store"
	"oreilly-cache/internal/upstream"
)

func main() {
	cacheDir        := flag.String("cache-dir", "./cache", "root directory for on-disk cache")
	listenAddr      := flag.String("listen", ":8080", "HTTP listen address")
	scrapeInterval  := flag.Duration("scrape-interval", time.Hour, "how often to re-scrape upstream")
	upstreamBase    := flag.String("upstream", "https://learning.oreilly.com", "upstream base URL")
	workers         := flag.Int("workers", 5, "max concurrent publisher-item scrapes")
	pageSize        := flag.Int("page-size", 100, "items per upstream page request")
	httpTimeout     := flag.Duration("http-timeout", 30*time.Second, "upstream HTTP client timeout per request")
	shutdownTimeout := flag.Duration("shutdown-timeout", 10*time.Second, "graceful shutdown deadline")
	flag.Parse()

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	st := store.New(*cacheDir)
	cl := upstream.New(*upstreamBase, &http.Client{Timeout: *httpTimeout})

	sc := scraper.New(st, cl, log, scraper.Config{
		Workers:  *workers,
		PageSize: *pageSize,
	})

	handler := server.NewHandler(st, cl, log)
	srv := &http.Server{
		Addr:         *listenAddr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Scraper runs in background; cancel it on shutdown.
	scraperCtx, scraperCancel := context.WithCancel(context.Background())
	defer scraperCancel()
	go sc.Run(scraperCtx, *scrapeInterval)

	// HTTP server in a separate goroutine so main can block on OS signal.
	serverErr := make(chan error, 1)
	go func() {
		log.Info("server listening", "addr", *listenAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-quit:
		log.Info("shutdown signal received", "signal", sig.String())
	case err := <-serverErr:
		log.Error("server error", "err", err)
		scraperCancel()
		os.Exit(1)
	}

	scraperCancel()

	shutCtx, shutCancel := context.WithTimeout(context.Background(), *shutdownTimeout)
	defer shutCancel()

	log.Info("shutting down", "timeout", *shutdownTimeout)
	if err := srv.Shutdown(shutCtx); err != nil {
		log.Error("shutdown error", "err", err)
		os.Exit(1)
	}
	log.Info("shutdown complete")
}
