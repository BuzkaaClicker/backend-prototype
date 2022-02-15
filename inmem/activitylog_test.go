package inmem

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/buzkaaclicker/buzza"
	"github.com/stretchr/testify/assert"
)

func TestActivityStore(t *testing.T) {
	assert := assert.New(t)
	ctx := context.Background()

	uid := buzza.UserId(5)

	s := NewActivityStore()
	{
		logs, err := s.ByUserId(ctx, uid, -1, 100)
		if assert.NoError(err) {
			assert.Equal(0, len(logs))
		}

		logs, err = s.ByUserId(ctx, uid, 1000, 100)
		if assert.NoError(err) {
			assert.Equal(0, len(logs))
		}
	}

	err := s.AddLog(ctx, uid, buzza.Activity{Name: "gdzie_ta_muza", Data: map[string]interface{}{"service": "sc"}})
	if !assert.NoError(err) {
		return
	}

	var firstLog buzza.ActivityLog
	{
		logs, err := s.ByUserId(ctx, uid, -1, 100)
		if !assert.NoError(err) || !assert.Equal(1, len(logs)) {
			return
		}
		firstLog = logs[0]
		assert.Equal("gdzie_ta_muza", firstLog.Name)
		assert.Equal(map[string]interface{}{"service": "sc"}, firstLog.Data)

		logs, err = s.ByUserId(ctx, uid, firstLog.Id, 100)
		if assert.NoError(err) {
			assert.Equal(0, len(logs))
		}

		logs, err = s.ByUserId(ctx, uid, firstLog.Id+1, 100)
		if assert.NoError(err) && assert.Equal(1, len(logs)) {
			assert.Equal(firstLog, logs[0])
		}

		logs, err = s.ByUserId(ctx, uid, math.MaxInt64, 100)
		if assert.NoError(err) && assert.Equal(1, len(logs)) {
			assert.Equal(firstLog, logs[0])
		}

		logs, err = s.ByUserId(ctx, uid, firstLog.Id-1, -5)
		if assert.NoError(err) {
			assert.Equal(0, len(logs))
		}

		logs, err = s.ByUserId(ctx, uid, firstLog.Id, -1)
		if assert.NoError(err) {
			assert.Equal(0, len(logs))
		}

		logs, err = s.ByUserId(ctx, uid, firstLog.Id+1, 1)
		if assert.NoError(err) && assert.Equal(1, len(logs)) {
			assert.Equal(firstLog, logs[0])
		}
	}

	{
		// unknown user id
		logs, err := s.ByUserId(ctx, buzza.UserId(34290), -1, 100)
		if assert.NoError(err) {
			assert.Equal(0, len(logs))
		}
	}

	for i := 0; i < 200; i++ {
		err := s.AddLog(ctx, uid, buzza.Activity{Name: "gdzie_ta_muza", Data: map[string]interface{}{"order": i}})
		if !assert.NoError(err) {
			return
		}
	}

	{
		logs, err := s.ByUserId(ctx, uid, -1, 100)
		if !assert.NoError(err) || !assert.Equal(100, len(logs)) {
			return
		}

		logs, err = s.ByUserId(ctx, uid, firstLog.Id, 100)
		fmt.Println("logs", firstLog.Id, logs)
		if assert.NoError(err) {
			assert.Equal(0, len(logs))
		}

		logs, err = s.ByUserId(ctx, uid, firstLog.Id+1, 100)
		if assert.NoError(err) {
			assert.Equal(1, len(logs))
			assert.Equal(firstLog, logs[0])
		}

		logs, err = s.ByUserId(ctx, uid, firstLog.Id+201, 200)
		if assert.NoError(err) {
			assert.Equal(200, len(logs))
			for i := 0; i < 200; i++ {
				assert.Equal(i, logs[len(logs)-1-i].Data["order"])
			}
		}

		logs, err = s.ByUserId(ctx, uid, firstLog.Id+200, 200)
		if assert.NoError(err) {
			assert.Equal(200, len(logs))
			for i := 0; i < 199; i++ {
				assert.Equal(199-i-1, logs[i].Data["order"])
			}
			assert.Equal(firstLog, logs[len(logs)-1])
		}
	}
}
