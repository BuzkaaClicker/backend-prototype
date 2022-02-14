package persistent

import (
	"context"
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
	store := &ActivityStore{DB: db}

	assert.NoError(store.AddLog(ctx, 1, buzza.Activity{Name: "logged in"}))
	assert.NoError(store.AddLog(ctx, 1, buzza.Activity{Name: "logged out",
		Data: map[string]interface{}{"jestem03": "albo96"}}))

	logs, err := store.ByUserId(ctx, 1)
	if !assert.NoError(err) {
		return
	}
	if !assert.Equal(2, len(logs)) {
		return
	}
	assert.Equal("logged in", logs[0].Name)
	assert.Equal("logged out", logs[1].Name)
	assert.Equal(map[string]interface{}{"jestem03": "albo96"}, logs[1].Data)
}
