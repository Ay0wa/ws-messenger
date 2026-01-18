package service

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"strings"

	"github.com/Ay0wa/ws-messenger/internal/models"
	"github.com/Ay0wa/ws-messenger/internal/repository"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

const inviteAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

var errInviteCollision = errors.New("invite code collision")

type ChatService struct {
	repo *repository.ChatRepo
}

func NewChatService(repo *repository.ChatRepo) *ChatService {
	return &ChatService{repo: repo}
}

func (s *ChatService) Create(ctx context.Context, name string, ownerID uuid.UUID) (models.Chat, error) {
	for i := 0; i < 3; i++ {
		code, err := newInviteCode(10)
		if err != nil {
			return models.Chat{}, err
		}

		chat := models.Chat{
			Name:       strings.TrimSpace(name),
			OwnerID:    ownerID,
			InviteCode: code,
		}

		created, err := s.repo.Create(ctx, chat, ownerID)
		if err == nil {
			return created, nil
		}

		if !isUniqueViolation(err) {
			return models.Chat{}, err
		}
	}

	return models.Chat{}, errInviteCollision
}

func newInviteCode(length int) (string, error) {
	if length <= 0 {
		return "", errors.New("invalid invite length")
	}

	b := make([]byte, length)
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(inviteAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = inviteAlphabet[n.Int64()]
	}
	return string(b), nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
