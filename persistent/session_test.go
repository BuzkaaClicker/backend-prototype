package persistent

import (
	"context"
	"testing"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/buntdb"
)

func TestSessionRegisterAndRefresh(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	db := PgOpenTest(ctx)
	defer db.Close()

	bdb, err := buntdb.Open(":memory:")
	if err != nil {
		panic(err)
	}
	defer bdb.Close()

	activityStore := inmem.NewActivityStore()
	sessionStore := &SessionStore{Buntdb: bdb, ActivityStore: &activityStore}

	session, err := sessionStore.RegisterNew(ctx, 9231982, "192.168.0.101", "Chrome/openBased")
	if !assert.NoError(err) {
		return
	}
	assert.Equal(buzza.UserId(9231982), session.UserId)
	assert.Equal("192.168.0.101", session.Ip)
	assert.Equal("Chrome/openBased", session.UserAgent)

	logs, err := activityStore.ByUserId(ctx, session.UserId, -1, 100)
	if !assert.NoError(err) {
		return
	}
	lastLog := logs[len(logs)-1]
	assert.Equal("session_created", lastLog.Name)
	assert.Equal("192.168.0.101", lastLog.Data["ip"])
	assert.Equal("Chrome/openBased", lastLog.Data["userAgent"])

	// test refresh without changes
	{
		session, err := sessionStore.AcquireAndRefresh(ctx, session.Token, "192.168.0.101", "Chrome/openBased")
		if !assert.NoError(err) {
			return
		}
		refreshedLogs, err := activityStore.ByUserId(ctx, session.UserId, -1, 100)
		if !assert.NoError(err) {
			return
		}
		// session refresh should not change logs
		assert.Equal(logs, refreshedLogs)
	}

	// test refresh with different ip
	{
		session, err := sessionStore.AcquireAndRefresh(ctx, session.Token, "192.168.0.102", "Chrome/openBased")
		if !assert.NoError(err) {
			return
		}
		refreshedLogs, err := activityStore.ByUserId(ctx, session.UserId, -1, 100)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(len(logs)+1, len(refreshedLogs))

		latestLog := refreshedLogs[0]
		if assert.Equal("session_changed_ip", latestLog.Name) {
			assert.Equal(session.Id, latestLog.Data["session_id"])
			assert.Equal("192.168.0.101", latestLog.Data["previous_ip"])
			assert.Equal("192.168.0.102", latestLog.Data["new_ip"])
		}
	}

	// test refresh with different user agent
	{
		session, err := sessionStore.AcquireAndRefresh(ctx, session.Token, "192.168.0.102", "Safari/macbockOS")
		if !assert.NoError(err) {
			return
		}
		refreshedLogs, err := activityStore.ByUserId(ctx, session.UserId, -1, 100)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(len(logs)+2, len(refreshedLogs))

		latestLog := refreshedLogs[0]
		if assert.Equal("session_changed_user_agent", latestLog.Name) {
			assert.Equal(session.Id, latestLog.Data["session_id"])
			assert.Equal("Chrome/openBased", latestLog.Data["previous_user_agent"])
			assert.Equal("Safari/macbockOS", latestLog.Data["new_user_agent"])
		}
	}
}

func Test_GenerateSessionTokenLength(t *testing.T) {
	assert := assert.New(t)

	token, err := generateSessionToken()
	if assert.NoError(err) {
		assert.True(len(token) > 20)
	}
}
