package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cbrophy/land_trakker/foundation/config"
	"github.com/cbrophy/land_trakker/foundation/web"
)

// multiHandler chains multiple slog handlers to write to all of them.
type multiHandler struct {
	handlers []slog.Handler
}

func (m *multiHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multiHandler) Handle(ctx context.Context, record slog.Record) error {
	for _, h := range m.handlers {
		_ = h.Handle(ctx, record)
	}
	return nil
}

func (m *multiHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithAttrs(attrs)
	}
	return &multiHandler{handlers: newHandlers}
}

func (m *multiHandler) WithGroup(name string) slog.Handler {
	newHandlers := make([]slog.Handler, len(m.handlers))
	for i, h := range m.handlers {
		newHandlers[i] = h.WithGroup(name)
	}
	return &multiHandler{handlers: newHandlers}
}

func main() {
	logCapture := web.NewLogCapture(1000)

	// Chain handlers: capture to LogCapture AND output JSON to stdout
	multiHandler := &multiHandler{
		handlers: []slog.Handler{
			logCapture,
			slog.NewJSONHandler(os.Stdout, nil),
		},
	}
	log := slog.New(multiHandler)

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "land_trakker.toml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Warn("config not found, using defaults", "path", cfgPath, "err", err)
		cfg = &config.Config{}
	}

	listen := cfg.Server.Listen
	if listen == "" {
		listen = ":8080"
	}

	srv := &http.Server{
		Addr:         listen,
		Handler:      newRouter(cfg, nil, nil, nil, nil, logCapture, nil, nil, nil),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Info("server starting", "addr", listen)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("shutdown error", "err", err)
	}
}
