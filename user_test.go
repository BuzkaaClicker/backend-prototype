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

	_, err := db.NewCreateTable().
		IfNotExists().
		Model((*User)(nil)).
		Exec(ctx)
	assert.NoError(err)

	_, err = db.NewInsert().
		Model(&User{
			DiscordId:           "123",
			DiscordRefreshToken: "123",
			Email:               "user@rol.es",
			RolesNames:          []string{"pro", "UNDEFINED role"},
		}).
		Exec(ctx)
	assert.NoError(err)

	var user User
	err = db.NewSelect().
		Model((*User)(nil)).
		Where("email=?", "user@rol.es").
		Scan(ctx, &user)
	assert.NoError(err)

	roles := user.Roles
	assert.Equal(roles, Roles{AllRoles["pro"]})
	assert.Equal(AccessAllowed, roles.Access(PermissionDownloadPro))
	assert.Equal(AccessUndefined, roles.Access(PermissionAdminDashboard))
}
