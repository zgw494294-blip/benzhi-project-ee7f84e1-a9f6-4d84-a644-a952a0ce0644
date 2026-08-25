package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"buoy-calibration-gate/internal/calibration"
	"buoy-calibration-gate/internal/httpapi"
	"buoy-calibration-gate/internal/repository"
)

func main() {
	if err := run(); err != nil {
		log.Printf("服务退出: %v", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := parseConfig()
	if err != nil {
		return err
	}
	ctx := context.Background()
	store, err := repository.Open(ctx, cfg.database)
	if err != nil {
		return fmt.Errorf("初始化 SQLite: %w", err)
	}
	defer store.Close()
	service := calibration.NewService(store)
	api := httpapi.New(service)
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return fmt.Errorf("监听 %s: %w", cfg.addr, err)
	}
	server := &http.Server{Handler: api.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second}
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	if cfg.selfcheck {
		ctx, cancel := context.WithTimeout(ctx, cfg.selfcheckTimeout)
		defer cancel()
		checkErr := runSelfcheck(ctx, listener.Addr().String())
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()
		shutdownErr := server.Shutdown(shutdownCtx)
		serverErr := <-serveErr
		if serverErr != nil && !errors.Is(serverErr, http.ErrServerClosed) {
			return serverErr
		}
		if checkErr != nil {
			return fmt.Errorf("selfcheck 失败: %w", checkErr)
		}
		if shutdownErr != nil {
			return shutdownErr
		}
		log.Printf("selfcheck 通过，监听地址 %s", listener.Addr().String())
		return nil
	}
	log.Printf("浮标校准放行服务监听 %s", listener.Addr().String())
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	select {
	case <-signalCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
