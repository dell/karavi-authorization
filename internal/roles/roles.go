package roles

import (
	"fmt"
)

type PoolQuota interface{}

type SystemID struct {
	PoolQuotas map[string]PoolQuota `json:"pool_quotas"`
}

func NewSystemID() *SystemID {
	return &SystemID{
		PoolQuotas: make(map[string]PoolQuota),
	}
}

type SystemType struct {
	SystemIDs map[string]*SystemID `json:"system_ids"`
}

func NewSystemType() *SystemType {
	return &SystemType{
		SystemIDs: make(map[string]*SystemID),
	}
}

type Role struct {
	SystemTypes map[string]*SystemType `json:"system_types"`
}

func NewRole() *Role {
	return &Role{
		SystemTypes: make(map[string]*SystemType),
	}
}

type JSON struct {
	Roles map[string]*Role `json:"roles"`
}

func NewJSON() *JSON {
	return &JSON{
		Roles: make(map[string]*Role),
	}
}

type Instance struct {
	Name       string
	SystemType string
	SystemID   string
	Pool       string
	Quota      string
}

func (r *JSON) Select(fn func(r Instance)) {
	for roleName, role := range r.Roles {
		for systemTypeName, systemType := range role.SystemTypes {
			for systemIDName, systemID := range systemType.SystemIDs {
				for poolName, poolQuota := range systemID.PoolQuotas {
					result := Instance{
						Name:       roleName,
						SystemType: systemTypeName,
						SystemID:   systemIDName,
						Pool:       poolName,
						Quota:      fmt.Sprintf("%v", poolQuota),
					}
					fn(result)
				}
			}
		}
	}
}

func (r *JSON) Instances() []Instance {
	var result []Instance

	r.Select(func(r Instance) {
		result = append(result, r)
	})

	return result
}

func (r *JSON) Add(is Instance) error {
	if r.Roles == nil {
		r.Roles = make(map[string]*Role)
	}
	if _, ok := r.Roles[is.Name]; !ok {
		r.Roles[is.Name] = NewRole()
	}
	if _, ok := r.Roles[is.Name].SystemTypes[is.SystemType]; !ok {
		r.Roles[is.Name].SystemTypes[is.SystemType] = NewSystemType()
	}
	if _, ok := r.Roles[is.Name].SystemTypes[is.SystemType].SystemIDs[is.SystemID]; !ok {
		r.Roles[is.Name].SystemTypes[is.SystemType].SystemIDs[is.SystemID] = NewSystemID()
	}
	if v, ok := r.Roles[is.Name].SystemTypes[is.SystemType].SystemIDs[is.SystemID].PoolQuotas[is.Pool]; ok {
		return fmt.Errorf("already exists: %v", v)
	}
	r.Roles[is.Name].SystemTypes[is.SystemType].SystemIDs[is.SystemID].PoolQuotas[is.Pool] = is.Quota
	return nil
}

func NewInstance(role string, parts ...string) Instance {
	ins := Instance{
		Name: role,
	}
	for i, v := range parts {
		switch i {
		case 0: // system type
			ins.SystemType = v
		case 1: // system id
			ins.SystemID = v
		case 2: // pool name
			ins.Pool = v
		case 3: // quota
			ins.Quota = v
		}

	}
	return ins
}
