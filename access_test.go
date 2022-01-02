package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccessMerge(t *testing.T) {
	assert := assert.New(t)

	assert.Equal(AccessAllowed, AccessUndefined.merge(AccessAllowed))
	assert.Equal(AccessAllowed, AccessAllowed.merge(AccessAllowed))
	assert.Equal(AccessAllowed, AccessAllowed.merge(AccessUndefined))
	assert.Equal(AccessAllowed, AccessForbidden.merge(AccessAllowed))

	assert.Equal(AccessUndefined, AccessUndefined.merge(AccessUndefined))

	assert.Equal(AccessForbidden, AccessUndefined.merge(AccessForbidden))
	assert.Equal(AccessForbidden, AccessForbidden.merge(AccessForbidden))
	assert.Equal(AccessForbidden, AccessForbidden.merge(AccessUndefined))
	assert.Equal(AccessForbidden, AccessAllowed.merge(AccessForbidden))

	rolePermitted := Role{Permissions: map[PermissionName]bool{PermissionDownloadPro: true}}
	roleUndefined := Role{Permissions: map[PermissionName]bool{}}
	roleForbidden := Role{Permissions: map[PermissionName]bool{PermissionDownloadPro: false}}
	allowedCases := []Roles{
		{
			rolePermitted,
		},
		{
			roleUndefined,
			rolePermitted,
			rolePermitted,
		},
		{
			rolePermitted,
			roleUndefined,
			rolePermitted,
		},
	}
	for _, equalCase := range allowedCases {
		assert.Equal(AccessAllowed, equalCase.Access(PermissionDownloadPro))
	}

	forbiddenCases := []Roles{
		{
			roleForbidden,
		},
		{
			roleUndefined,
			rolePermitted,
			roleForbidden,
		},
		{
			rolePermitted,
			roleUndefined,
			roleForbidden,
		},
		{
			roleForbidden,
			roleUndefined,
			roleForbidden,
		},
		{
			roleForbidden,
			roleForbidden,
		},
		{
			roleForbidden,
			rolePermitted,
			roleForbidden,
		},
	}
	for i, equalCase := range forbiddenCases {
		assert.Equal(AccessForbidden, equalCase.Access(PermissionDownloadPro), "access index: %d", i)
	}
}

func TestMapRolesById(t *testing.T) {
	assert := assert.New(t)

	roleAdmin := Role{
		Id: RoleIdAdmin,
		Permissions: map[PermissionName]bool{
			PermissionDownloadPro:    true,
			PermissionAdminDashboard: true,
		},
	}

	rolePro := Role{
		Id: RoleIdPro,
		Permissions: map[PermissionName]bool{
			PermissionDownloadPro: true,
		},
	}

	rolesMapped := mapRolesById(roleAdmin, rolePro)
	assert.Equal(roleAdmin, rolesMapped[RoleIdAdmin])
	assert.Equal(rolePro, rolesMapped[RoleIdPro])

	assert.Panics(func() {
		mapRolesById(roleAdmin, roleAdmin)
	})
	assert.Panics(func() {
		mapRolesById(roleAdmin, rolePro, roleAdmin)
	})
	assert.Panics(func() {
		mapRolesById(rolePro, rolePro, roleAdmin)
	})
}
