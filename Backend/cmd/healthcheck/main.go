// Command healthcheck probes the local admin listener from a distroless image.
package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	_, port, err := net.SplitHostPort(os.Getenv("ADMIN_ADDR"))
	if err != nil {
		return fmt.Errorf("invalid ADMIN_ADDR")
	}
	numericPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || numericPort == 0 {
		return fmt.Errorf("invalid admin port")
	}
	port = strconv.FormatUint(numericPort, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	// #nosec G704 -- destination host is fixed loopback; only a validated uint16 port is configurable.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort("127.0.0.1", port)+"/readyz", nil)
	if err != nil {
		return err
	}
	// #nosec G704 -- request URL is restricted to loopback and a validated numeric admin port above.
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("readiness status %d", response.StatusCode)
	}
	return nil
}
