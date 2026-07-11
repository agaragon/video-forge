// Command server runs the video-forge API + encoding worker pool described
// in Kickoff.md §6.
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/agaragon/video-forge/internal/api"
	"github.com/agaragon/video-forge/internal/config"
	"github.com/agaragon/video-forge/internal/job"
	"github.com/agaragon/video-forge/internal/media"
	"github.com/agaragon/video-forge/internal/store"
)

func main() {
	cfg := config.Default()

	fileStore, err := store.New(cfg.UploadDir, cfg.OutputDir)
	if err != nil {
		log.Fatalf("initializing storage: %v", err)
	}

	prober := media.NewProber(cfg.FFprobePath)
	encoder := media.NewEncoder(cfg.FFmpegPath)

	manager := job.NewManager(encoder, cfg.WorkerCount, cfg.JobTimeout, cfg.OutputRetention)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	manager.Start(ctx)

	srv := api.NewServer(cfg, prober, fileStore, manager, "web/static")
	httpServer := &http.Server{
		Addr:    cfg.Addr,
		Handler: srv.Routes(),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("http shutdown: %v", err)
		}
	}()

	log.Printf("video-forge listening on %s (workers=%d, ffmpeg=%s)", cfg.Addr, cfg.WorkerCount, cfg.FFmpegPath)
	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server: %v", err)
	}

	manager.Wait()
}
