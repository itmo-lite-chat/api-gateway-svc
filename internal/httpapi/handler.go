package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/itmo-lite-chat/api-gateway-svc/internal/domain"
	"github.com/itmo-lite-chat/api-gateway-svc/internal/messages"
	"github.com/itmo-lite-chat/api-gateway-svc/internal/store"
)

type Handler struct {
	store    *store.Store
	messages *messages.Client
}

func NewHandler(store *store.Store, messages *messages.Client) *Handler {
	return &Handler{store: store, messages: messages}
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

	user, ok := h.store.Login(req.Username, req.Password)
	if !ok {
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

	chats := h.store.ListChats(user)
	resp := make([]domain.Chat, 0, len(chats))
	for _, chat := range chats {
		resp = append(resp, h.toHTTPChat(chat, user))
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

	chat := h.store.GetOrCreatePrivateChat(user, req.Username)
	writeJSON(w, http.StatusOK, map[string]domain.Chat{"chat": h.toHTTPChat(chat, user)})
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

	chat, ok := h.store.ChatByID(parts[0])
	if !ok || !contains(chat.Participants, user.ID) {
		writeError(w, http.StatusNotFound, "chat not found")
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
	h.store.TouchChat(chat.ID, msg.Text, timestamp)

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
	user, ok := h.store.UserByToken(token)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return domain.User{}, false
	}
	return user, true
}

func (h *Handler) toHTTPChat(chat domain.StoredChat, current domain.User) domain.Chat {
	name := "Chat"
	initials := "C"
	for _, participantID := range chat.Participants {
		if participantID == current.ID {
			continue
		}
		user, ok := h.store.UserByID(participantID)
		if ok {
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
