package db

import (
	"context"

	"github.com/Ay0wa/ws-messenger/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	return pgxpool.New(ctx, cfg.DBURL())
}
