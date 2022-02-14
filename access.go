package buzza

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

type RoleId string

type Role struct {
	Id          RoleId
	Permissions map[PermissionName]bool
}

var (
	RoleIdPro   RoleId = "pro"
	RoleIdAdmin RoleId = "admin"
)

var AllRoles map[RoleId]Role = mapRolesById(
	Role{
		Id: RoleIdAdmin,
		Permissions: map[PermissionName]bool{
			PermissionDownloadPro:    true,
			PermissionAdminDashboard: true,
		},
	},
	Role{
		Id: RoleIdPro,
		Permissions: map[PermissionName]bool{
			PermissionDownloadPro: true,
		},
	},
)

func mapRolesById(roles ...Role) map[RoleId]Role {
	rolesMap := make(map[RoleId]Role)
	for _, role := range roles {
		if _, ok := rolesMap[role.Id]; ok {
			panic("Duplicated role id: `" + role.Id + "`!")
		}
		rolesMap[role.Id] = role
	}
	return rolesMap
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
