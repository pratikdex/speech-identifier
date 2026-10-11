package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"dictation/internal/providers"
	"dictation/internal/providers/google"
	"dictation/internal/providers/sarvam"
	"dictation/internal/server"
	"dictation/internal/service"
)

func main() {
	port := flag.String("port", "8081", "Server port")
	languages := flag.String("stt-languages", "en-IN,hi-IN,ml-IN,kn-IN", "Google STT primary language and up to 3 alternatives")
	origin := flag.String("origin", "http://localhost:3000", "Browser origin allowed to open sessions")
	flag.Parse()

	// Load API keys from .env if present; real environment variables win.
	_ = godotenv.Load()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Order of this list is the default priority; STT_PROVIDER moves one STT to the front.
	var all []any
	for _, f := range []providers.Factory{google.Factory(strings.Split(*languages, ",")), sarvam.Factory} {
		ps, err := f(ctx)
		if err != nil {
			log.Fatal(err)
		}
		all = append(all, ps...)
	}

	// A new capability (e.g. text-to-speech) is one more providers.Of line here.
	cfg := service.Config{
		STT:         providers.PreferFirst(providers.Of[providers.SpeechToText](all), os.Getenv("STT_PROVIDER")),
		Translators: providers.Of[providers.Translator](all),
	}

	srv := &http.Server{
		Addr:        ":" + *port,
		Handler:     server.NewHandler(service.New(cfg), *origin),
		BaseContext: func(net.Listener) context.Context { return ctx }, // shutdown closes sessions
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	log.Printf("listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
