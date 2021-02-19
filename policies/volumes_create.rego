package karavi.volumes.create

import data.karavi.common

default quota = 0
quota = common.roles[token.role].quota

default response = {
  "allowed": true
}
response = {
    "allowed": false,
    "status": {
      "reason": reason,
    },
} {
  reason = concat(", ", deny)
  reason != ""
}

deny[msg] {
  common.roles == {}
  msg := sprintf("no role data found", [])
}

deny[msg] {
  quota == 0
  msg := sprintf("zero quota for request", [])
}

deny[msg] {
  token == {}
  msg := sprintf("token was invalid", [])
}

deny[msg] {
  not common.roles[token.role]
  msg := sprintf("unknown role: %q", [token.role])
}

deny[msg] {
  checked_system_role_entry = {}
  msg := sprintf("role %v does not have access to storage system %v", [input.role, input.storagesystemid])
}


deny[msg] {
  checked_pool_entry = {}
  msg := sprintf("role %v does not have access to storage pool %v on storage system %v", [input.role, input.storagepool, input.storagesystemid])
}

deny[msg] {
  quota := checked_pool_entry.quota
  cap := to_number(input.request.volumeSizeInKb)
  cap > quota
  msg := sprintf("requested capacity %v exceeds quota %v for role %q on storage pool %v on storage system %v", [format_int(cap,10), format_int(quota,10), input.role, input.storagepool, input.storagesystemid])
}

default checked_system_role_entry = {}
checked_system_role_entry = v{
  some system_role_entry
  roles[input.role][system_role_entry].storage_system_id = input.storagesystemid
  v = roles[input.role][system_role_entry]
}

default checked_pool_entry = {}
checked_pool_entry = v{
  some pool_entry
  checked_system_role_entry.pool_quotas[pool_entry].pool = input.storagepool
  v = checked_system_role_entry.pool_quotas[pool_entry]
}

default token = {}
token = payload {
  [valid, _, payload] := io.jwt.decode_verify(input.token, {"secret": common.secret, "aud": "karavi"})
  valid == true
}
