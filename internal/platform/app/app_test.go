package app_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"pulsebid/internal/platform/app"
	"pulsebid/internal/platform/config"
)

func settings(address string) config.Common {
	return config.Common{ServiceName: "test", LogLevel: "info", AdminAddr: address, OTLPEndpoint: "localhost:1", TraceSampleRatio: 0, ShutdownDelay: 100 * time.Millisecond, ShutdownTimeout: time.Second}
}

func TestRunReadinessAndShutdown(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	reserve, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := reserve.Addr().String()
	require.NoError(t, reserve.Close())
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx, settings(address), "dev", io.Discard) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Error("app did not stop")
		}
	})
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	statusCode := func(path string) int {
		requestCtx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		req, err := http.NewRequestWithContext(requestCtx, "GET", "http://"+address+path, nil)
		if err != nil {
			return 0
		}
		res, err := client.Do(req)
		if err != nil {
			return 0
		}
		defer func() { require.NoError(t, res.Body.Close()) }()
		_, err = io.Copy(io.Discard, res.Body)
		require.NoError(t, err)
		return res.StatusCode
	}
	require.Eventually(t, func() bool { return statusCode("/readyz") == 200 }, 3*time.Second, 5*time.Millisecond)
	require.Equal(t, 200, statusCode("/healthz"))
	require.Equal(t, 200, statusCode("/metrics"))
	cancel()
	require.Eventually(t, func() bool { return statusCode("/readyz") == 503 }, 80*time.Millisecond, time.Millisecond)
	require.Equal(t, 200, statusCode("/healthz"))
}

func TestInvalidConfigAndOccupiedPort(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.Error(t, app.Run(ctx, config.Common{}, "dev", io.Discard))
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { require.NoError(t, listener.Close()) }()
	require.ErrorContains(t, app.Run(ctx, settings(listener.Addr().String()), "dev", io.Discard), "listen admin")
}
