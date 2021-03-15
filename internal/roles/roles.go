package roles

import (
	"fmt"
)

// PoolQuota represents a quota for a storage pool.
// The empty interface value is used to support the
// json.Decoder#UseNumber function.
type PoolQuota interface{}

// SystemID is associated with a system ID and references
// the associated pool quotas.
type SystemID struct {
	PoolQuotas map[string]PoolQuota `json:"pool_quotas"`
}

// NewSystemID allocates a new SystemID.
func NewSystemID() *SystemID {
	return &SystemID{
		PoolQuotas: make(map[string]PoolQuota),
	}
}

// SystemType is associated with a system type (e.g. powerflex)
// and references any system IDs.
type SystemType struct {
	SystemIDs map[string]*SystemID `json:"system_ids"`
}

// NewSystemType allocates a new SystemType.
func NewSystemType() *SystemType {
	return &SystemType{
		SystemIDs: make(map[string]*SystemID),
	}
}

// Role is associated with a role name and references any
// system types.
type Role struct {
	SystemTypes map[string]*SystemType `json:"system_types"`
}

// NewRole allocates a new Role.
func NewRole() *Role {
	return &Role{
		SystemTypes: make(map[string]*SystemType),
	}
}

// JSON represents the outermost struct for marshaling
// a roles JSON payload.
type JSON struct {
	Roles map[string]*Role `json:"roles"`
}

// NewJSON allocates a new JSON.
func NewJSON() *JSON {
	return &JSON{
		Roles: make(map[string]*Role),
	}
}

// Instance represents a unique Role instance from
// within a collection of roles.
type Instance struct {
	Name       string
	SystemType string
	SystemID   string
	Pool       string
	Quota      string
}

// Select iterates through each individual role Instance and
// passes it to the provided predicate function for custom
// selection purposes.
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

// Instances returns a slice of Instances that are guaranteed to
// be unique.
func (r *JSON) Instances() []Instance {
	var result []Instance

	r.Select(func(r Instance) {
		result = append(result, r)
	})

	return result
}

// Add inserts the given Instance into the JSON structure.
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
	r.Roles[is.Name].SystemTypes[is.SystemType].SystemIDs[is.SystemID].PoolQuotas[is.Pool] = is.Quota
	return nil
}

// NewInstance is a convenience function for creating a new
// role based on a slice of string tokens.
//
// Example:
//   role=roleA
//   parts=[]string{powerflex,12345,mypool,100G}
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
