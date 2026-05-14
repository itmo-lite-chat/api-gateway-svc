package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/itmo-lite-chat/api-gateway-svc/internal/chats"
	"github.com/itmo-lite-chat/api-gateway-svc/internal/domain"
	"github.com/itmo-lite-chat/api-gateway-svc/internal/messages"
	"github.com/itmo-lite-chat/api-gateway-svc/internal/users"
)

type Handler struct {
	messages   *messages.Client
	users      *users.Client
	chatClient *chats.Client
}

func NewHandler(messages *messages.Client, users *users.Client, chats *chats.Client) *Handler {
	return &Handler{messages: messages, users: users, chatClient: chats}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", h.health)
	mux.HandleFunc("/api/auth/login", h.login)
	mux.HandleFunc("/api/me", h.me)
	mux.HandleFunc("/api/chats", h.chats)
	mux.HandleFunc("/api/chats/private", h.privateChat)
	mux.HandleFunc("/api/chats/", h.chatNested)
	return h.cors(mux)
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	user, err := h.users.Login(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	writeJSON(w, http.StatusOK, map[string]domain.User{"user": user})
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, ok := h.authUser(w, r)
	if !ok {
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (h *Handler) chats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, ok := h.authUser(w, r)
	if !ok {
		return
	}

	userChats, err := h.chatClient.List(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	resp := make([]domain.Chat, 0, len(userChats))
	for _, chat := range userChats {
		resp = append(resp, h.toHTTPChat(r.Context(), chat, user))
	}

	writeJSON(w, http.StatusOK, map[string][]domain.Chat{"chats": resp})
}

func (h *Handler) privateChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	user, ok := h.authUser(w, r)
	if !ok {
		return
	}

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	participant, err := h.users.GetOrCreateByLogin(r.Context(), req.Username)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	chat, err := h.chatClient.GetOrCreatePrivate(r.Context(), user.ID, participant.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]domain.Chat{"chat": h.toHTTPChat(r.Context(), chat, user)})
}

func (h *Handler) chatNested(w http.ResponseWriter, r *http.Request) {
	user, ok := h.authUser(w, r)
	if !ok {
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/chats/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[1] != "messages" {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	ok, err := h.chatClient.CheckMember(r.Context(), parts[0], user.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, "chat not found")
		return
	}

	chat, err := h.chatClient.Get(r.Context(), parts[0])
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getMessages(w, r, chat)
	case http.MethodPost:
		h.sendMessage(w, r, chat, user)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getMessages(w http.ResponseWriter, r *http.Request, chat domain.StoredChat) {
	msgs, err := h.messages.History(r.Context(), chat.ID)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	resp := make([]domain.Message, 0, len(msgs))
	for _, msg := range msgs {
		resp = append(resp, domain.Message{
			ID:        msg.ID,
			SenderID:  msg.SenderID,
			Text:      msg.Text,
			Timestamp: msg.Timestamp,
		})
	}

	writeJSON(w, http.StatusOK, map[string][]domain.Message{"messages": resp})
}

func (h *Handler) sendMessage(w http.ResponseWriter, r *http.Request, chat domain.StoredChat, user domain.User) {
	var req struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	msg, err := h.messages.Send(r.Context(), chat.ID, user.ID, req.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}

	timestamp := msg.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	messageID, err := strconv.ParseInt(msg.ID, 10, 64)
	if err == nil {
		_ = h.chatClient.TouchLastMessage(r.Context(), chat.ID, messageID, msg.Text)
	}

	writeJSON(w, http.StatusOK, map[string]domain.Message{"message": {
		ID:        msg.ID,
		SenderID:  msg.SenderID,
		Text:      msg.Text,
		Timestamp: timestamp,
	}})
}

func (h *Handler) authUser(w http.ResponseWriter, r *http.Request) (domain.User, bool) {
	auth := r.Header.Get("Authorization")
	token := strings.TrimPrefix(auth, "Bearer ")
	user, err := h.users.ValidateToken(r.Context(), token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return domain.User{}, false
	}
	return user, true
}

func (h *Handler) toHTTPChat(ctx context.Context, chat domain.StoredChat, current domain.User) domain.Chat {
	name := "Chat"
	initials := "C"
	for _, participantID := range chat.Participants {
		if participantID == current.ID {
			continue
		}
		user, err := h.users.GetByID(ctx, participantID)
		if err == nil {
			name = user.DisplayName
			initials = avatarInitials(user.DisplayName)
		}
	}

	return domain.Chat{
		ID:              chat.ID,
		Name:            name,
		AvatarInitials:  initials,
		LastMessage:     chat.LastMessage,
		LastMessageTime: chat.LastMessageAt,
		UnreadCount:     0,
		IsOnline:        true,
	}
}

func (h *Handler) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func avatarInitials(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	return strings.ToUpper(name[:1])
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
