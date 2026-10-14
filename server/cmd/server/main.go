package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	speech "cloud.google.com/go/speech/apiv1"
	stt "github.com/pratikdex/speech-identifier"
	"github.com/pratikdex/speech-identifier/providers"
	"github.com/pratikdex/speech-identifier/providers/google"
)

func main() {
	// Parse command line flags
	port := flag.String("port", "8081", "Server port")
	flag.Parse()

	// Create providers based on flags
	var provider providers.Provider
	var cleanupFuncs []func() error

	p, cleanup, err := createGoogleProvider()
	if err != nil {
		log.Printf("Failed to create Google provider: %v", err)
		return
	}
	provider = p
	if cleanup != nil {
		cleanupFuncs = append(cleanupFuncs, cleanup)
	}

	// Cleanup all providers on exit
	defer func() {
		for _, cleanup := range cleanupFuncs {
			if err := cleanup(); err != nil {
				log.Printf("Error during provider cleanup: %v", err)
			}
		}
	}()

	// Create server with all providers
	s := stt.New(*port, provider)

	go func() {
		if err := s.Start(); err != nil {
			log.Fatalf("Server failed to start: %v\n", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	if err := s.Stop(); err != nil {
		log.Printf("Error during server shutdown: %v\n", err)
	}
}

func createGoogleProvider() (providers.Provider, func() error, error) {
	if os.Getenv("GOOGLE_APPLICATION_CREDENTIALS") == "" {
		return nil, nil, fmt.Errorf("GOOGLE_APPLICATION_CREDENTIALS environment variable is required for Google provider")
	}

	ctx := context.Background()
	speechClient, err := speech.NewClient(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Google Speech client: %v", err)
	}

	provider := google.NewProvider(speechClient)

	return provider, speechClient.Close, nil
}
