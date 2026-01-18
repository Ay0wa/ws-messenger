package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ay0wa/ws-messenger/internal/auth"
	authmw "github.com/Ay0wa/ws-messenger/internal/middleware"
	"github.com/Ay0wa/ws-messenger/internal/models"
	"github.com/Ay0wa/ws-messenger/internal/repository"
	"github.com/Ay0wa/ws-messenger/internal/service"
	"github.com/Ay0wa/ws-messenger/internal/ws"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

type Handler struct {
	users *repository.UserRepo
	chats *repository.ChatRepo
	auth  *auth.Service
	svc   *service.ChatService
	hub   *ws.Hub
}

func NewHandler(users *repository.UserRepo, chats *repository.ChatRepo, authSvc *auth.Service, chatSvc *service.ChatService, hub *ws.Hub) *Handler {
	return &Handler{
		users: users,
		chats: chats,
		auth:  authSvc,
		svc:   chatSvc,
		hub:   hub,
	}
}

func (h *Handler) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(chimw.Logger)

	r.Get("/health", h.health)
	r.Get("/swagger/*", httpSwagger.WrapHandler)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.register)
		r.Post("/login", h.login)
	})

	r.Group(func(r chi.Router) {
		r.Use(authmw.RequireAuth(h.auth))
		r.Route("/chats", func(r chi.Router) {
			r.Post("/", h.createChat)
			r.Get("/", h.listChats)
			r.Post("/join/{code}", h.joinChat)
			r.Get("/{id}/messages", h.listMessages)
		})
		r.Get("/ws/chats/{id}", h.chatWS)
	})

	return r
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

type authResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// register godoc
// @Summary Регистрация
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body registerRequest true "Данные регистрации"
// @Success 201 {object} authResponse
// @Failure 400 {object} errorResponse
// @Failure 409 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /auth/register [post]
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.Email == "" || req.Password == "" || req.DisplayName == "" {
		writeError(w, http.StatusBadRequest, "missing fields")
		return
	}

	if _, err := h.users.GetByEmail(r.Context(), req.Email); err == nil {
		writeError(w, http.StatusConflict, "email already registered")
		return
	} else if !errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusInternalServerError, "failed to check user")
		return
	}

	hash, err := h.auth.HashPassword(req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user, err := h.users.Create(r.Context(), models.User{
		Email:        req.Email,
		PasswordHash: hash,
		DisplayName:  req.DisplayName,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	token, err := h.auth.IssueToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	writeJSON(w, http.StatusCreated, authResponse{
		Token: token,
		User:  toUserResponse(user),
	})
}

// login godoc
// @Summary Логин
// @Tags auth
// @Accept json
// @Produce json
// @Param payload body loginRequest true "Данные логина"
// @Success 200 {object} authResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /auth/login [post]
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "missing fields")
		return
	}

	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := h.auth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.auth.IssueToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}

	writeJSON(w, http.StatusOK, authResponse{
		Token: token,
		User:  toUserResponse(user),
	})
}

type createChatRequest struct {
	Name string `json:"name"`
}

type chatResponse struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	OwnerID    uuid.UUID `json:"owner_id"`
	InviteCode string    `json:"invite_code"`
	CreatedAt  time.Time `json:"created_at"`
}

// createChat godoc
// @Summary Создать чат
// @Tags chats
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param payload body createChatRequest true "Данные чата"
// @Success 201 {object} chatResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /chats [post]
func (h *Handler) createChat(w http.ResponseWriter, r *http.Request) {
	var req createChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing name")
		return
	}

	userID, ok := authmw.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user")
		return
	}

	chat, err := h.svc.Create(r.Context(), req.Name, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chat")
		return
	}

	writeJSON(w, http.StatusCreated, toChatResponse(chat))
}

// listChats godoc
// @Summary Список чатов пользователя
// @Tags chats
// @Security BearerAuth
// @Produce json
// @Success 200 {array} chatResponse
// @Failure 401 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /chats [get]
func (h *Handler) listChats(w http.ResponseWriter, r *http.Request) {
	userID, ok := authmw.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user")
		return
	}

	chats, err := h.chats.ListByUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list chats")
		return
	}

	resp := make([]chatResponse, 0, len(chats))
	for _, chat := range chats {
		resp = append(resp, toChatResponse(chat))
	}

	writeJSON(w, http.StatusOK, resp)
}

// joinChat godoc
// @Summary Вступить в чат по инвайт-коду
// @Tags chats
// @Security BearerAuth
// @Produce json
// @Param code path string true "Invite code"
// @Success 200 {object} chatResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /chats/join/{code} [post]
func (h *Handler) joinChat(w http.ResponseWriter, r *http.Request) {
	code := chi.URLParam(r, "code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing code")
		return
	}

	userID, ok := authmw.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user")
		return
	}

	chat, err := h.chats.GetByInviteCode(r.Context(), code)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get chat")
		return
	}

	if err := h.chats.AddMember(r.Context(), chat.ID, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to join chat")
		return
	}

	writeJSON(w, http.StatusOK, toChatResponse(chat))
}

type messageResponse struct {
	ID        uuid.UUID `json:"id"`
	ChatID    uuid.UUID `json:"chat_id"`
	UserID    uuid.UUID `json:"user_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// listMessages godoc
// @Summary История сообщений
// @Tags messages
// @Security BearerAuth
// @Produce json
// @Param id path string true "Chat ID"
// @Param limit query int false "Limit"
// @Param before query string false "RFC3339 timestamp"
// @Success 200 {array} messageResponse
// @Failure 400 {object} errorResponse
// @Failure 401 {object} errorResponse
// @Failure 403 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Router /chats/{id}/messages [get]
func (h *Handler) listMessages(w http.ResponseWriter, r *http.Request) {
	userID, ok := authmw.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user")
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	isMember, err := h.chats.IsMember(r.Context(), chatID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}
	if !isMember {
		writeError(w, http.StatusForbidden, "not a member")
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 200 {
			limit = parsed
		}
	}

	before := time.Now()
	if v := r.URL.Query().Get("before"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			before = t
		}
	}

	messages, err := h.chats.ListMessages(r.Context(), chatID, limit, before)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list messages")
		return
	}

	resp := make([]messageResponse, 0, len(messages))
	for _, msg := range messages {
		resp = append(resp, messageResponse{
			ID:        msg.ID,
			ChatID:    msg.ChatID,
			UserID:    msg.UserID,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

type wsIncoming struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

type wsOutgoing struct {
	Type string          `json:"type"`
	Data messageResponse `json:"data"`
}

// chatWS godoc
// @Summary WebSocket подключение
// @Tags ws
// @Security BearerAuth
// @Param id path string true "Chat ID"
// @Success 101 {string} string "Switching Protocols"
// @Router /ws/chats/{id} [get]
func (h *Handler) chatWS(w http.ResponseWriter, r *http.Request) {
	userID, ok := authmw.UserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing user")
		return
	}

	chatID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	isMember, err := h.chats.IsMember(r.Context(), chatID, userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check membership")
		return
	}
	if !isMember {
		writeError(w, http.StatusForbidden, "not a member")
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{"*"}})
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	client := &ws.Client{
		Conn:   conn,
		ChatID: chatID,
		UserID: userID,
		Send:   make(chan []byte, 16),
	}

	h.hub.Register(client)
	defer h.hub.Unregister(client)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go client.WriteLoop(ctx)

	for {
		var in wsIncoming
		if err := wsjson.Read(ctx, conn, &in); err != nil {
			return
		}
		if strings.ToLower(in.Type) != "message" {
			continue
		}
		content := strings.TrimSpace(in.Content)
		if content == "" {
			continue
		}

		msg, err := h.chats.AddMessage(r.Context(), models.Message{
			ChatID:  chatID,
			UserID:  userID,
			Content: content,
		})
		if err != nil {
			continue
		}

		payload := wsOutgoing{
			Type: "message",
			Data: messageResponse{
				ID:        msg.ID,
				ChatID:    msg.ChatID,
				UserID:    msg.UserID,
				Content:   msg.Content,
				CreatedAt: msg.CreatedAt,
			},
		}
		data, err := json.Marshal(payload)
		if err != nil {
			continue
		}

		h.hub.Broadcast(chatID, data)
	}
}

func toUserResponse(user models.User) userResponse {
	return userResponse{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
	}
}

func toChatResponse(chat models.Chat) chatResponse {
	return chatResponse{
		ID:         chat.ID,
		Name:       chat.Name,
		OwnerID:    chat.OwnerID,
		InviteCode: chat.InviteCode,
		CreatedAt:  chat.CreatedAt,
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
