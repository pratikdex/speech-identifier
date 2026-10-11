package providers

import (
	"context"
	"log"
	"sort"
)

// Factory builds the provider objects a vendor offers (SpeechToText,
// Translator, or any capability added later; one object may implement
// several), reading its own config. A vendor that isn't configured returns
// nothing. Adding a vendor is one package with a Factory plus one line in
// cmd/server/main.go.
//
// shortcut: clients are never closed, the OS reclaims them at exit; add a
// Close to the Factory result if a vendor needs a graceful shutdown.
type Factory func(ctx context.Context) ([]any, error)

// Of returns the items implementing T, in order.
func Of[T any](all []any) []T {
	var out []T
	for _, p := range all {
		if t, ok := p.(T); ok {
			out = append(out, t)
		}
	}
	return out
}

// PreferFirst moves the provider named first to the front, keeping the rest
// in order. An unknown name is logged and ignored.
func PreferFirst[T interface{ Name() string }](list []T, first string) []T {
	p, err := ParseProvider(first)
	if err != nil && first != "" {
		log.Printf("ignoring preferred provider %q: unknown provider", first)
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Name() == p.String() && list[j].Name() != p.String()
	})
	return list
}
