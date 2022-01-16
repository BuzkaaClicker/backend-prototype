package main

import (
	"context"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProfileLookup(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	app := createTestApp()

	profile := &Profile{
		UserId:    1,
		Name:      "ww_makin_c",
		AvatarUrl: "https://buzkaaclicker.pl/avatar/123",
	}
	_, err := app.profileStore.DB.NewInsert().
		Model(profile).
		On("CONFLICT (user_id) DO UPDATE SET " +
			"user_id=EXCLUDED.user_id, name=EXCLUDED.name, avatar_url=EXCLUDED.avatar_url").
		Exec(ctx)
	if !assert.NoError(err) {
		return
	}

	t.Run("lookup store", func(t *testing.T) {
		profileSel, err := app.profileStore.ByUserId(ctx, 1)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(profile.UserId, profileSel.UserId)
		assert.Equal(profile.Name, profileSel.Name)
		assert.Equal(profile.AvatarUrl, profileSel.AvatarUrl)
	})

	t.Run("controller lookup", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/profile/1", nil)
		resp, err := app.server.Test(req)
		if !assert.NoError(err) {
			return
		}
		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(`{"name":"ww_makin_c","avatarUrl":"https://buzkaaclicker.pl/avatar/123"}`, string(body))
	})
}
