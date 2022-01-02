package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserRoles(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
		return
	}
	assert := assert.New(t)
	ctx := context.Background()

	app := createTestApp()

	_, err := app.db.NewCreateTable().
		IfNotExists().
		Model((*User)(nil)).
		Exec(ctx)
	assert.NoError(err)

	_, err = app.db.NewInsert().
		Model(&User{
			DiscordId:           "123",
			DiscordRefreshToken: "123",
			Email:               "user@rol.es",
			RolesNames:          []RoleId{RoleIdPro, RoleId("UNDEFINED role")},
		}).
		Exec(ctx)
	assert.NoError(err)

	var user User
	err = app.db.NewSelect().
		Model((*User)(nil)).
		Where("email=?", "user@rol.es").
		Scan(ctx, &user)
	assert.NoError(err)

	roles := user.Roles
	assert.Equal(Roles{AllRoles[RoleIdPro]}, roles)
	assert.Equal(AccessAllowed, roles.Access(PermissionDownloadPro))
	assert.Equal(AccessUndefined, roles.Access(PermissionAdminDashboard))
}
