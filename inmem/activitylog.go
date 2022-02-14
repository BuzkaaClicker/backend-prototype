package inmem

import (
	"context"
	"sync"
	"time"

	"github.com/buzkaaclicker/buzza"
)

type ActivityStore struct {
	lastId int64
	logs   map[buzza.UserId][]buzza.ActivityLog
	mutex  sync.RWMutex
}

func NewActivityStore() ActivityStore {
	return ActivityStore{
		lastId: 0,
		logs:   make(map[buzza.UserId][]buzza.ActivityLog),
		mutex:  sync.RWMutex{},
	}
}

func (s *ActivityStore) AddLog(ctx context.Context, userId buzza.UserId, activity buzza.Activity) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	ulogs, ok := s.logs[userId]
	if !ok {
		ulogs = make([]buzza.ActivityLog, 0, 10)
	}
	s.lastId++
	ulogs = append(ulogs, buzza.ActivityLog{
		Id:        s.lastId,
		CreatedAt: time.Time{},
		UserId:    userId,
		Name:      activity.Name,
		Data:      activity.Data,
	})
	s.logs[userId] = ulogs
	return nil
}

func (s *ActivityStore) ByUserId(ctx context.Context, userId buzza.UserId) ([]buzza.ActivityLog, error) {
	s.mutex.RLock()
	logs, ok := s.logs[userId]
	s.mutex.RUnlock()
	if ok {
		return logs, nil
	} else {
		return []buzza.ActivityLog{}, nil
	}
}
