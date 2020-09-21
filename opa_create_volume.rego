package dell.policy

import data.dell.quotas
import input

default allow = false

allow {
  ten_cap := quotas.tenants[input.cluster].namespaces[input.pvc_namespace].capacity_quota_in_kb
  cap := input.capacity
  cap < ten_cap
}
