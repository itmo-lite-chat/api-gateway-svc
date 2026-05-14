package store

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/itmo-lite-chat/api-gateway-svc/internal/domain"
)

var namespace = uuid.MustParse("12fb1769-c9f8-4f3d-9f2e-2b7de1d41831")

type Store struct {
	mu     sync.RWMutex
	secret []byte
	users  map[string]domain.User
	chats  map[string]domain.StoredChat
}

func New(secret string) *Store {
	return &Store{
		secret: []byte(secret),
		users:  make(map[string]domain.User),
		chats:  make(map[string]domain.StoredChat),
	}
}

func (s *Store) Login(username, password string) (domain.User, bool) {
	if password != "aboba" || strings.TrimSpace(username) == "" {
		return domain.User{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	user := s.getOrCreateUserLocked(username)
	user.Token = s.sign(user.ID)
	return user, true
}

func (s *Store) UserByToken(token string) (domain.User, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return domain.User{}, false
	}

	userIDBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return domain.User{}, false
	}

	userID := string(userIDBytes)
	if !hmac.Equal([]byte(parts[1]), []byte(s.signature(userID))) {
		return domain.User{}, false
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.ID == userID {
			user.Token = token
			return user, true
		}
	}

	return domain.User{}, false
}

func (s *Store) GetOrCreatePrivateChat(owner domain.User, username string) domain.StoredChat {
	s.mu.Lock()
	defer s.mu.Unlock()

	participant := s.getOrCreateUserLocked(username)
	ids := []string{owner.ID, participant.ID}
	if ids[0] > ids[1] {
		ids[0], ids[1] = ids[1], ids[0]
	}

	chatID := uuid.NewSHA1(namespace, []byte(ids[0]+":"+ids[1])).String()
	chat, ok := s.chats[chatID]
	if !ok {
		chat = domain.StoredChat{
			ID:            chatID,
			Participants:  ids,
			LastMessageAt: time.Now(),
		}
		s.chats[chatID] = chat
	}

	return chat
}

func (s *Store) ListChats(user domain.User) []domain.StoredChat {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chats := make([]domain.StoredChat, 0)
	for _, chat := range s.chats {
		if contains(chat.Participants, user.ID) {
			chats = append(chats, chat)
		}
	}

	return chats
}

func (s *Store) ChatByID(chatID string) (domain.StoredChat, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	chat, ok := s.chats[chatID]
	return chat, ok
}

func (s *Store) TouchChat(chatID, text string, at time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	chat, ok := s.chats[chatID]
	if !ok {
		return
	}
	chat.LastMessage = text
	chat.LastMessageAt = at
	s.chats[chatID] = chat
}

func (s *Store) UserByID(userID string) (domain.User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, user := range s.users {
		if user.ID == userID {
			return user, true
		}
	}
	return domain.User{}, false
}

func (s *Store) getOrCreateUserLocked(username string) domain.User {
	username = strings.TrimSpace(username)
	user, ok := s.users[username]
	if ok {
		return user
	}

	user = domain.User{
		ID:          uuid.NewSHA1(namespace, []byte("user:"+username)).String(),
		Username:    username,
		DisplayName: username,
	}
	s.users[username] = user
	return user
}

func (s *Store) sign(userID string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(userID)) + "." + s.signature(userID)
}

func (s *Store) signature(userID string) string {
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(userID))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
