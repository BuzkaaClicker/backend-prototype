package persistent

import (
	"context"
	"testing"

	"github.com/buzkaaclicker/buzza"
	"github.com/buzkaaclicker/buzza/discord"
	"github.com/stretchr/testify/assert"
)

func TestUserRoles(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	db := PgOpenTest(ctx)
	defer db.Close()

	_, err := db.NewInsert().
		Model(&User{
			DiscordId:           "1235",
			DiscordRefreshToken: "123",
			Email:               "user@rol.es",
			RolesNames:          []buzza.RoleId{buzza.RoleIdPro, buzza.RoleId("UNDEFINED role")},
		}).
		Exec(ctx)
	if !assert.NoError(err) {
		return
	}

	var user User
	err = db.NewSelect().
		Model((*User)(nil)).
		Where("email=?", "user@rol.es").
		Scan(ctx, &user)
	assert.NoError(err)

	roles := user.Roles
	assert.Equal(buzza.Roles{buzza.AllRoles[buzza.RoleIdPro]}, roles)
	assert.Equal(buzza.AccessAllowed, roles.Access(buzza.PermissionDownloadPro))
	assert.Equal(buzza.AccessUndefined, roles.Access(buzza.PermissionAdminDashboard))
}

func TestRegisterDiscordUser(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	db := PgOpenTest(ctx)
	defer db.Close()
	store := UserStore{DB: db}

	discordUser := discord.User{
		Id:         "snowflake",
		Username:   "www_makin_cc",
		Email:      "clickacz@discord.makin.cc",
		AvatarHash: "f2789ef0ddaee56d91a782fa530b0009",
	}
	refreshToken := "21gokpoasio57"
	user, err := store.RegisterDiscordUser(ctx, discordUser, refreshToken)
	if !assert.NoError(err) {
		return
	}
	assert.Equal(discordUser.Id, user.Discord.Id)
	assert.Equal(refreshToken, user.Discord.RefreshToken)
	assert.Equal(discordUser.Email, string(user.Email))

	userSel, err := store.ById(ctx, user.Id)
	if !assert.NoError(err) {
		return
	}
	assert.Equal(user, userSel)
}
