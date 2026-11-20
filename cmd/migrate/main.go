// Command migrate applies embedded PostgreSQL migrations.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"pulsebid/migrations"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: migrate up|down")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL is required; make migrate-up/down loads the local environment")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	return migrations.Apply(ctx, dsn, os.Args[1])
}
