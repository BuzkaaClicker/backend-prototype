package profile

import (
	"context"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	"github.com/buzkaaclicker/backend/pgdb"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestProfileLookup(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	db := pgdb.OpenTest(ctx)
	defer db.Close()

	store := Store {
		DB: db,
	}

	profile := &Model{
		UserId:    1,
		Name:      "ww_makin_c",
		AvatarUrl: "https://buzkaaclicker.pl/avatar/123",
	}
	_, err := store.DB.NewInsert().
		Model(profile).
		On("CONFLICT (user_id) DO UPDATE SET " +
			"user_id=EXCLUDED.user_id, name=EXCLUDED.name, avatar_url=EXCLUDED.avatar_url").
		Exec(ctx)
	if !assert.NoError(err) {
		return
	}

	t.Run("lookup store", func(t *testing.T) {
		profileSel, err := store.ByUserId(ctx, 1)
		if !assert.NoError(err) {
			return
		}
		assert.Equal(profile.UserId, profileSel.UserId)
		assert.Equal(profile.Name, profileSel.Name)
		assert.Equal(profile.AvatarUrl, profileSel.AvatarUrl)
	})

	t.Run("controller lookup", func(t *testing.T) {
		controller := Controller{
			Store: store,
		}
		app := fiber.New()
		controller.Install(app)

		req := httptest.NewRequest("GET", "/profile/1", nil)
		resp, err := app.Test(req)
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
