package persistent

import (
	"context"
	"math"
	"testing"

	"github.com/buzkaaclicker/buzza"
	"github.com/stretchr/testify/assert"
)

func TestActivityStore(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()
	db := PgOpenTest(ctx)
	defer db.Close()

	_, err := db.NewDelete().
		Model((*ActivityLog)(nil)).
		Where("1=1").
		Exec(ctx)
	if !assert.NoError(err) {
		return
	}

	store := &ActivityStore{DB: db}

	const uid = 1

	assert.NoError(store.AddLog(ctx, uid, buzza.Activity{Name: "logged in"}))
	assert.NoError(store.AddLog(ctx, uid, buzza.Activity{Name: "logged out",
		Data: map[string]interface{}{"jestem03": "albo96"}}))

	var lastLog buzza.ActivityLog
	{
		logs, err := store.ByUserId(ctx, uid, -1, 100)
		if !assert.NoError(err) {
			return
		}
		if !assert.Equal(2, len(logs)) {
			return
		}
		lastLog = logs[len(logs)-1]
		assert.Equal("logged out", logs[0].Name)
		assert.Equal(map[string]interface{}{"jestem03": "albo96"}, logs[0].Data)
		assert.Equal("logged in", logs[1].Name)
	}

	{
		logs, err := store.ByUserId(ctx, uid, lastLog.Id, 100)
		if !assert.NoError(err) {
			assert.Equal(1, len(logs))
		}

		logs, err = store.ByUserId(ctx, uid, lastLog.Id+1, 100)
		if !assert.NoError(err) {
			assert.Equal(2, len(logs))
		}

		logs, err = store.ByUserId(ctx, uid, math.MaxInt64, 100)
		if !assert.NoError(err) {
			assert.Equal(2, len(logs))
		}

		logs, err = store.ByUserId(ctx, uid, lastLog.Id-2, -5)
		if !assert.NoError(err) {
			assert.Equal(0, len(logs))
		}

		logs, err = store.ByUserId(ctx, uid, lastLog.Id+2, -6)
		if !assert.NoError(err) {
			assert.Equal(0, len(logs))
		}
	}
}
