# Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http:#www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

package karavi.volumes.create

import data.karavi.common

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

#
# Ensure there are roles configured.
#
deny[msg] {
  common.roles == {}
  msg := sprintf("no role data found", [])
}

#
# Ensure the requested role exists.
#
deny[msg] {
  not common.roles[claims.role]
  msg := sprintf("unknown role: %q", [claims.role])
}

#
# Validate input: claims.
#
default claims = {}
claims = input.claims
deny[msg] {                                                                                       
  claims == {}
  msg := sprintf("missing claims", [])

#
# Validate input: storagesystemid.
#
default storagesystemid = ""
storagesystemid = input.storagesystemid
deny[msg] {
  storagesystemid = ""
  msg := sprintf("invalid storage system id requested", [])
}

#
# Validate input: storagepool.
#
default storagepool = ""
storagepool = input.storagepool
deny[msg] {
  storagepool = ""
  msg := sprintf("invalid storage pool requested", [])
}

#
# Check and get the requested storage system.
#
default checked_system_role_entry = {}
checked_system_role_entry = v {
  some system_role_entry
  common.roles[claims.role][system_role_entry].storage_system_id = storagesystemid
  v = common.roles[claims.role][system_role_entry]
}
deny[msg] {
  checked_system_role_entry = {}
  msg := sprintf("role %v does not have access to storage system %v", [claims.role, input.storagesystemid])
}

#
# Check and get the requested storage pool.
#
default checked_pool_entry = {}
checked_pool_entry = v {
  some pool_entry
  checked_system_role_entry.pool_quotas[pool_entry].pool = input.storagepool
  v = checked_system_role_entry.pool_quotas[pool_entry]
}
deny[msg] {
  checked_pool_entry = {}
  msg := sprintf("role %v does not have access to storage pool %v on storage system %v", [claims.role, input.storagepool, input.storagesystemid])
}

#
# Get the quota for the OPA response
#
default quota = 0
quota = v {
  v = checked_pool_entry.quota
}

#
# Ensure the requested capacity does not exceed the quota.
#
deny[msg] {
  quota := checked_pool_entry.quota
  cap := to_number(input.request.volumeSizeInKb)
  cap > quota
  msg := sprintf("requested capacity %v exceeds quota %v for role %q on storage pool %v on storage system %v", [
    format_int(cap,10),
    format_int(quota,10),
    claims.role,
    input.storagepool,
    input.storagesystemid])
}
