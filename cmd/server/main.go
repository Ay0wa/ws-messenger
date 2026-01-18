package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	_ "github.com/Ay0wa/ws-messenger/api/docs"
	"github.com/Ay0wa/ws-messenger/internal/auth"
	"github.com/Ay0wa/ws-messenger/internal/config"
	"github.com/Ay0wa/ws-messenger/internal/db"
	"github.com/Ay0wa/ws-messenger/internal/httpapi"
	"github.com/Ay0wa/ws-messenger/internal/repository"
	"github.com/Ay0wa/ws-messenger/internal/service"
	"github.com/Ay0wa/ws-messenger/internal/ws"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
)

// @title ws-messenger API
// @version 0.1.0
// @description WebSocket messenger API
// @host localhost:8080
// @BasePath /
// @schemes http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	cfg := config.MustLoad()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	}))

	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg)
	if err != nil {
		logger.Error("db connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	authSvc := auth.NewService(cfg.JWTSecret, cfg.JWTTTLMinutes)
	userRepo := repository.NewUserRepo(pool)
	chatRepo := repository.NewChatRepo(pool)
	chatSvc := service.NewChatService(chatRepo)
	hub := ws.NewHub()

	handler := httpapi.NewHandler(userRepo, chatRepo, authSvc, chatSvc, hub)

	root := chi.NewRouter()
	root.Use(cors.Handler(cors.Options{
		AllowedOrigins:   parseOrigins(cfg.CORSOrigins),
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	root.Mount("/", handler.Routes())

	srv := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           root,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("server starting", "addr", cfg.Addr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = srv.Shutdown(ctxShutdown)
	logger.Info("server stopped")
}

func parseOrigins(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{"*"}
	}
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		origins = append(origins, p)
	}
	if len(origins) == 0 {
		return []string{"*"}
	}
	return origins
}
