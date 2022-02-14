package inmem

import (
	"context"
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
		logs, err := s.ByUserId(ctx, uid)
		if assert.NoError(err) {
			assert.Equal(0, len(logs))
		}
	}

	err := s.AddLog(ctx, uid, buzza.Activity{Name: "gdzie_ta_muza", Data: map[string]interface{}{"service": "sc"}})
	if !assert.NoError(err) {
		return
	}

	{
		logs, err := s.ByUserId(ctx, uid)
		if !assert.NoError(err) {
			return
		}

		if !assert.Equal(1, len(logs)) {
			return
		}
		log := logs[0]
		assert.Equal("gdzie_ta_muza", log.Name)
		assert.Equal(map[string]interface{}{"service": "sc"}, log.Data)
	}

	{
		// unknown user id
		logs, err := s.ByUserId(ctx, buzza.UserId(34290))
		if assert.NoError(err) {
			assert.Equal(0, len(logs))
		}
	}
}
