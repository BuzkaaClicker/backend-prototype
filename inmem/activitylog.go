package inmem

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/buzkaaclicker/buzza"
)

type ActivityStore struct {
	lastId int64
	logs   map[buzza.UserId][]buzza.ActivityLog
	mutex  sync.RWMutex
}

var _ buzza.ActivityStore = (*ActivityStore)(nil)

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

func (s *ActivityStore) ByUserId(ctx context.Context, userId buzza.UserId, beforeId int64, limit int32) ([]buzza.ActivityLog, error) {
	const maxLimit = 10_000
	if limit > maxLimit {
		return nil, fmt.Errorf("too big limit %d/%d", limit, maxLimit)
	}
	if limit <= 0 {
		return []buzza.ActivityLog{}, nil
	}

	s.mutex.RLock()
	logs, ok := s.logs[userId]

	lastBeforeIdIndex := len(logs)
	if beforeId >= 0 {
		for i, l := range logs {
			if beforeId <= l.Id {
				lastBeforeIdIndex = i
				break
			}
		}
	}
	startIndex := lastBeforeIdIndex - int(limit)
	if startIndex < 0 {
		startIndex = 0
	}
	filteredLogs := make([]buzza.ActivityLog, lastBeforeIdIndex - int(startIndex))
	// copy in a reversed order
	for i := range filteredLogs {
		filteredLogs[i] = logs[lastBeforeIdIndex - i - 1]
	}
	
	s.mutex.RUnlock()

	if ok {
		return filteredLogs, nil
	} else {
		return []buzza.ActivityLog{}, nil
	}
}
