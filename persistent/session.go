package persistent

import (
	"context"
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/buzkaaclicker/buzza"
	"github.com/google/uuid"
	"github.com/tidwall/buntdb"
)

const sessionTTL = 30 * 24 * time.Hour // 30 days

type Session struct {
	Id             string    `json:"id"`
	UserId         int64     `json:"userId"`
	Token          string    `json:"token"`
	Ip             string    `json:"ip"`
	UserAgent      string    `json:"userAgent"`
	LastAccessedAt time.Time `json:"lastAccessedAt"`
	ExpiresAt      time.Time `json:"expiresAt"`
}

func (s Session) ToDomain() buzza.Session {
	return buzza.Session{
		Id:             s.Id,
		UserId:         buzza.UserId(s.UserId),
		Token:          s.Token,
		Ip:             s.Ip,
		UserAgent:      s.UserAgent,
		LastAccessedAt: s.LastAccessedAt,
		ExpiresAt:      s.ExpiresAt,
	}
}

type SessionStore struct {
	Buntdb        *buntdb.DB
	ActivityStore buzza.ActivityStore
}

func (s *SessionStore) CreateIndexes() {
	s.Buntdb.CreateIndex("sessions", "session:*", buntdb.IndexString)
}

func (s *SessionStore) RegisterNew(ctx context.Context, userId buzza.UserId, ip string, userAgent string) (buzza.Session, error) {
	token, err := generateSessionToken()
	if err != nil {
		return buzza.Session{}, fmt.Errorf("generate token: %s", err)
	}
	id := uuid.New().String()

	err = s.ActivityStore.AddLog(ctx, userId, buzza.Activity{Name: "session_created", Data: map[string]interface{}{
		"ip":         ip,
		"userAgent":  userAgent,
		"session_id": id,
	}})
	if err != nil {
		return buzza.Session{}, fmt.Errorf("add session_created activity log: %s", err)
	}

	session := Session{
		Id:             id,
		UserId:         int64(userId),
		Token:          token,
		Ip:             ip,
		UserAgent:      userAgent,
		LastAccessedAt: time.Now().UTC(),
		ExpiresAt:      time.Now().UTC().Add(sessionTTL),
	}
	serializedSession, err := json.Marshal(&session)
	if err != nil {
		return buzza.Session{}, fmt.Errorf("session serialize: %s", err)
	}

	err = s.Buntdb.Update(func(tx *buntdb.Tx) error {
		expireOptions := &buntdb.SetOptions{Expires: true, TTL: sessionTTL}

		_, replaced, err := tx.Set("session_by_id:"+session.Id, session.Token, expireOptions)
		if err != nil {
			return fmt.Errorf("set map session id to auth token: %w", err)
		}
		if replaced {
			return fmt.Errorf("rarest uuid collision '%s' (not possible)", session.Id)
		}

		_, _, err = tx.Set("session:"+session.Token, string(serializedSession), expireOptions)
		if err != nil {
			return fmt.Errorf("set session: %w", err)
		}
		return nil
	})
	if err != nil {
		return buzza.Session{}, fmt.Errorf("bunt update: %s", err)
	}
	return session.ToDomain(), nil
}

func (s *SessionStore) ByToken(token string) (buzza.Session, error) {
	var session Session
	err := s.Buntdb.View(func(tx *buntdb.Tx) error {
		serializedSession, err := tx.Get("session:" + token)
		if err != nil {
			return fmt.Errorf("get serialized session: %w", err)
		}
		if err := json.Unmarshal([]byte(serializedSession), &session); err != nil {
			return fmt.Errorf("deserialize session: %s", err)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, buntdb.ErrNotFound) {
			return buzza.Session{}, buzza.ErrSessionNotFound
		} else {
			return buzza.Session{}, fmt.Errorf("buntdb view: %s", err)
		}
	}
	return session.ToDomain(), err
}

func (s *SessionStore) Exists(token string) (bool, error) {
	err := s.Buntdb.View(func(tx *buntdb.Tx) error {
		_, err := tx.Get("session:" + token)
		return err
	})
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, buntdb.ErrNotFound):
		return false, nil
	default:
		return false, fmt.Errorf("bunt view: %s", err)
	}
}

func (s *SessionStore) activeSessions(tx *buntdb.Tx, token string) ([]buzza.Session, error) {
	sessions := make([]buzza.Session, 0, 10)
	var listErr error
	err := tx.Ascend("sessions", func(key, value string) bool {
		var session Session
		if err := json.Unmarshal([]byte(value), &session); err != nil {
			listErr = fmt.Errorf("deserialize session: %s", err)
			return false
		}
		sessions = append(sessions, session.ToDomain())
		return true
	})
	if err != nil {
		return nil, fmt.Errorf("ascend sessions: %w", err)
	}
	if listErr != nil {
		return nil, fmt.Errorf("ascend content sessions: %w", listErr)
	}
	return sessions, nil
}

func (s *SessionStore) ActiveSessions(token string) ([]buzza.Session, error) {
	sessions := make([]buzza.Session, 0, 10)
	err := s.Buntdb.View(func(tx *buntdb.Tx) error {
		var err error
		sessions, err = s.activeSessions(tx, token)
		if err != nil {
			return fmt.Errorf("lookup active sessions: %w", err)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, buntdb.ErrNotFound) {
			return nil, buzza.ErrSessionNotFound
		} else {
			return nil, fmt.Errorf("buntdb view: %s", err)
		}
	}
	return sessions, nil
}

func (s *SessionStore) AcquireAndRefresh(ctx context.Context, token string, ip string, userAgent string) (buzza.Session, error) {
	var previousSession Session
	var session Session
	err := s.Buntdb.View(func(tx *buntdb.Tx) error {
		oldSerializedSession, err := tx.Get("session:" + token)
		if err != nil {
			return fmt.Errorf("get serialized session: %w", err)
		}
		err = json.Unmarshal([]byte(oldSerializedSession), &previousSession)
		if err != nil {
			return fmt.Errorf("deserialize session: %w", err)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, buntdb.ErrNotFound) {
			return buzza.Session{}, buzza.ErrSessionNotFound
		} else {
			return buzza.Session{}, fmt.Errorf("get session from buntdb: %s", err)
		}
	}

	// copy session
	session = previousSession
	session.Ip = ip
	session.UserAgent = userAgent
	session.LastAccessedAt = time.Now().UTC()
	session.ExpiresAt = time.Now().UTC().Add(sessionTTL)
	serializedSession, err := json.Marshal(session)
	if err != nil {
		return buzza.Session{}, fmt.Errorf("serialize session: %s", err)
	}

	err = s.Buntdb.Update(func(tx *buntdb.Tx) error {
		serializedSessionStr := string(serializedSession)
		_, _, err = tx.Set("session:"+token, serializedSessionStr, &buntdb.SetOptions{Expires: true, TTL: sessionTTL})
		if err != nil {
			return fmt.Errorf("store session: %w", err)
		}

		// nwm nie podobaja mi sie te zapytania do db w tym locku buntowym
		// todo moze jakas zmiana
		if previousSession.Ip != session.Ip {
			activity := buzza.Activity{Name: "session_changed_ip", Data: map[string]interface{}{
				"session_id":  session.Id,
				"previous_ip": previousSession.Ip,
				"new_ip":      session.Ip,
			}}
			if err := s.ActivityStore.AddLog(ctx, buzza.UserId(session.UserId), activity); err != nil {
				return fmt.Errorf("log ip change: %s", err)
			}
		}
		if previousSession.UserAgent != session.UserAgent {
			activity := buzza.Activity{Name: "session_changed_user_agent", Data: map[string]interface{}{
				"session_id":          session.Id,
				"previous_user_agent": previousSession.UserAgent,
				"new_user_agent":      session.UserAgent,
			}}
			if err := s.ActivityStore.AddLog(ctx, buzza.UserId(session.UserId), activity); err != nil {
				return fmt.Errorf("log useragent change: %s", err)
			}
		}
		return nil
	})
	if err != nil {
		return buzza.Session{}, fmt.Errorf("refresh session in buntdb: %s", err)
	}
	return session.ToDomain(), nil
}

func (s *SessionStore) InvalidateById(userId buzza.UserId, sessionId string) error {
	err := s.Buntdb.Update(func(tx *buntdb.Tx) error {
		token, err := tx.Get("session_by_id:" + sessionId)
		if err != nil {
			return fmt.Errorf("get session by id: %w", err)
		}
		serializedSession, err := tx.Delete("session:" + token)
		if err != nil {
			return fmt.Errorf("delete session by auth token: %w", err)
		}

		var session Session
		err = json.Unmarshal([]byte(serializedSession), &session)
		if err != nil {
			return fmt.Errorf("deserialize session: %w", err)
		}
		if userId != buzza.UserId(session.UserId) {
			return fmt.Errorf("different user id (required: %d, found: %d)", userId, session.UserId)
		}

		_, err = tx.Delete("session_by_id:" + sessionId)
		if err != nil {
			return fmt.Errorf("delete session by id: %w", err)
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("bunt update: %w", err)
	}
	return nil
}

func (s *SessionStore) InvalidateByAuthToken(authToken string) error {
	err := s.Buntdb.Update(func(tx *buntdb.Tx) error {
		serializedSession, err := tx.Delete("session:" + authToken)
		if err != nil {
			return fmt.Errorf("delete session key: %w", err)
		}
		var session Session
		err = json.Unmarshal([]byte(serializedSession), &session)
		if err != nil {
			return fmt.Errorf("deserialize deleted session: %w", err)
		}
		_, err = tx.Delete("session_by_id:" + session.Id)
		if err != nil {
			return fmt.Errorf("delete session key: %w", err)
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("bunt update: %s", err)
	}
	return nil
}

func (s *SessionStore) InvalidateAllExpect(expectToken string) error {
	err := s.Buntdb.Update(func(tx *buntdb.Tx) error {
		sessions, err := s.activeSessions(tx, expectToken)
		if err != nil {
			return fmt.Errorf("ascend sessions: %w", err)
		}
		for _, session := range sessions {
			if session.Token == expectToken {
				continue
			}

			_, err = tx.Delete("session_by_id:" + session.Id)
			if err != nil {
				return fmt.Errorf("delete session_by_id: %w", err)
			}
			_, err = tx.Delete("session:" + session.Token)
			if err != nil {
				return fmt.Errorf("delete session: %w", err)
			}
		}
		return err
	})
	if err != nil {
		return fmt.Errorf("bunt update: %s", err)
	}
	return nil
}

func generateSessionToken() (string, error) {
	const tokenBytes = 60
	rawToken := make([]byte, tokenBytes)
	// crypto/rand - getentropy(2)
	bytesRead, err := crand.Read(rawToken)
	if err != nil {
		return "", fmt.Errorf("rand read: %w", err)
	}
	if bytesRead != tokenBytes {
		return "", fmt.Errorf("bytes read %d / required %d", bytesRead, tokenBytes)
	}
	dirtyToken := base64.StdEncoding.EncodeToString(rawToken)

	// replace all ":" with "_" to make our
	// session store queries at buntdb BUNTDBinjection safe
	// (for example, if later someone will add key "session:token:random_sufix" then
	// without line below theorically it can overwrite this random sufix)
	token := strings.Replace(dirtyToken, ":", "_", -1)
	return token, nil
}
