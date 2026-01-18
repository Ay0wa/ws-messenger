package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Ay0wa/ws-messenger/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ChatRepo struct {
	db *pgxpool.Pool
}

func NewChatRepo(db *pgxpool.Pool) *ChatRepo {
	return &ChatRepo{db: db}
}

func (r *ChatRepo) Create(ctx context.Context, chat models.Chat, ownerID uuid.UUID) (models.Chat, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return models.Chat{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	chat.OwnerID = ownerID

	query := `
        INSERT INTO chats (name, owner_id, invite_code)
        VALUES ($1, $2, $3)
        RETURNING id, created_at
    `
	err = tx.QueryRow(ctx, query, chat.Name, ownerID, chat.InviteCode).Scan(&chat.ID, &chat.CreatedAt)
	if err != nil {
		return models.Chat{}, err
	}

	memberQuery := `
        INSERT INTO chat_members (chat_id, user_id, role)
        VALUES ($1, $2, $3)
    `
	_, err = tx.Exec(ctx, memberQuery, chat.ID, ownerID, "owner")
	if err != nil {
		return models.Chat{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return models.Chat{}, err
	}

	return chat, nil
}

func (r *ChatRepo) GetByInviteCode(ctx context.Context, code string) (models.Chat, error) {
	query := `
        SELECT id, name, owner_id, invite_code, created_at
        FROM chats
        WHERE invite_code = $1
    `
	var chat models.Chat
	err := r.db.QueryRow(ctx, query, code).Scan(
		&chat.ID,
		&chat.Name,
		&chat.OwnerID,
		&chat.InviteCode,
		&chat.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Chat{}, pgx.ErrNoRows
	}
	return chat, err
}

func (r *ChatRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Chat, error) {
	query := `
        SELECT c.id, c.name, c.owner_id, c.invite_code, c.created_at
        FROM chats c
        JOIN chat_members m ON m.chat_id = c.id
        WHERE m.user_id = $1
        ORDER BY c.created_at DESC
    `
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chats []models.Chat
	for rows.Next() {
		var chat models.Chat
		if err := rows.Scan(&chat.ID, &chat.Name, &chat.OwnerID, &chat.InviteCode, &chat.CreatedAt); err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	return chats, rows.Err()
}

func (r *ChatRepo) AddMember(ctx context.Context, chatID, userID uuid.UUID) error {
	query := `
        INSERT INTO chat_members (chat_id, user_id, role)
        VALUES ($1, $2, $3)
        ON CONFLICT (chat_id, user_id) DO NOTHING
    `
	_, err := r.db.Exec(ctx, query, chatID, userID, "member")
	return err
}

func (r *ChatRepo) IsMember(ctx context.Context, chatID, userID uuid.UUID) (bool, error) {
	query := `
        SELECT 1
        FROM chat_members
        WHERE chat_id = $1 AND user_id = $2
    `
	var exists int
	err := r.db.QueryRow(ctx, query, chatID, userID).Scan(&exists)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return true, err
}

func (r *ChatRepo) AddMessage(ctx context.Context, msg models.Message) (models.Message, error) {
	query := `
        INSERT INTO messages (chat_id, user_id, content)
        VALUES ($1, $2, $3)
        RETURNING id, created_at
    `
	err := r.db.QueryRow(ctx, query, msg.ChatID, msg.UserID, msg.Content).Scan(&msg.ID, &msg.CreatedAt)
	return msg, err
}

func (r *ChatRepo) ListMessages(ctx context.Context, chatID uuid.UUID, limit int, before time.Time) ([]models.Message, error) {
	query := `
        SELECT id, chat_id, user_id, content, created_at
        FROM messages
        WHERE chat_id = $1 AND created_at < $2
        ORDER BY created_at DESC
        LIMIT $3
    `
	rows, err := r.db.Query(ctx, query, chatID, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(&msg.ID, &msg.ChatID, &msg.UserID, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}
