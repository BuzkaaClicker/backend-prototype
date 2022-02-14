package persistent

import (
	"context"
	"testing"

	"github.com/buzkaaclicker/buzza"
	"github.com/stretchr/testify/assert"
)

func TestProfileServiceCreate(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	db := PgOpenTest(ctx)
	defer db.Close()

	service := &ProfileService{
		DB: db,
	}

	user := &User{RolesNames: []buzza.RoleId{}, DiscordId: "23904321095490", DiscordRefreshToken: "missing"}
	_, err := service.DB.NewInsert().
		Model(user).
		Ignore().
		Exec(ctx)
	if !assert.NoError(err) {
		return
	}
	profile := &Profile{
		UserId:    user.Id,
		User:      user,
		Name:      "ww_makin_c",
		AvatarUrl: "https://buzkaaclicker.pl/avatar/123",
	}

	_, err = service.DB.NewInsert().
		Model(profile).
		On("CONFLICT (user_id) DO UPDATE SET " +
			"user_id=EXCLUDED.user_id, name=EXCLUDED.name, avatar_url=EXCLUDED.avatar_url").
		Exec(ctx)
	if !assert.NoError(err) {
		return
	}

	profileSel, err := service.ByUserId(ctx, buzza.UserId(user.Id))
	if !assert.NoError(err) {
		return
	}
	assert.Equal(profile.User.Id, int64(profileSel.User.Id))
	assert.Equal(profile.Name, profileSel.Name)
	assert.Equal(profile.AvatarUrl, profileSel.AvatarUrl)
}
