package main

import (
	"github.com/gofiber/fiber/v2"
)

type Access byte

const (
	AccessUndefined Access = 0
	AccessForbidden Access = 1
	AccessAllowed   Access = 2
)

func (a Access) merge(b Access) Access {
	switch {
	case a == AccessUndefined:
		return b
	case b == AccessUndefined:
		return a
	default:
		return b
	}
}

type PermissionName string

const (
	PermissionDownloadPro    PermissionName = "download.pro"
	PermissionAdminDashboard PermissionName = "admin.dashboard"
)

type Role struct {
	Name        string
	Permissions map[PermissionName]bool
}

var AllRoles = map[string]Role{
	"pro": {
		Name: "pro",
		Permissions: map[PermissionName]bool{
			PermissionDownloadPro: true,
		},
	},
	"admin": {
		Name: "admin",
		Permissions: map[PermissionName]bool{
			PermissionDownloadPro:    true,
			PermissionAdminDashboard: true,
		},
	},
}

func (role Role) Access(name PermissionName) Access {
	hasPermission, ok := role.Permissions[name]
	switch {
	case !ok:
		return AccessUndefined
	case hasPermission:
		return AccessAllowed
	default:
		return AccessForbidden
	}
}

type Roles []Role

func (roles Roles) Access(permission PermissionName) Access {
	access := AccessUndefined
	for _, role := range roles {
		access = access.merge(role.Access(permission))
	}
	return access
}

func RequirePermissions(permission PermissionName) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		user, ok := ctx.Locals(UserKey).(*User)
		if !ok {
			return fiber.ErrUnauthorized
		}
		if user.Roles.Access(permission) != AccessAllowed {
			return fiber.ErrUnauthorized
		}
		return nil
	}
}
