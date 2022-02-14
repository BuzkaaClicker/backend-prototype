package inmem

import (
	"context"
	"sync"
	"time"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/discord"
)

type UserStore struct {
	lastId int64
	users  map[buzza.UserId]buzza.User
	mutex  sync.RWMutex
}

func NewUserStore() UserStore {
	return UserStore{
		lastId: 0,
		users:  map[buzza.UserId]buzza.User{},
		mutex:  sync.RWMutex{},
	}
}

func (s *UserStore) RegisterDiscordUser(ctx context.Context, u discord.User, refreshToken string) (buzza.User, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.lastId++
	uid := buzza.UserId(s.lastId)
	user := buzza.User{
		Id:        uid,
		CreatedAt: time.Now(),
		Roles:     []buzza.Role{},
		Discord: buzza.UserDiscord{
			Id:           u.Id,
			RefreshToken: refreshToken,
		},
		Email: buzza.Email(u.Email),
	}
	s.users[uid] = user

	return user, nil
}

func (s *UserStore) ById(ctx context.Context, userId buzza.UserId) (buzza.User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	u, ok := s.users[userId]
	if !ok {
		return u, buzza.ErrUserNotFound
	}
	return u, nil
}

func (s *UserStore) ByDiscordId(ctx context.Context, discordId string) (buzza.User, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, u := range s.users {
		if u.Discord.Id == discordId {
			return u, nil
		}
	}
	return buzza.User{}, buzza.ErrUserNotFound
}

func (s *UserStore) Update(ctx context.Context, user buzza.User) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.users[user.Id] = user
	return nil
}
