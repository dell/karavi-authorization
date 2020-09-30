package dell.create_volume

import data.dell.quotas
import input

default allow = {"result": false}

allow = {"provisional_cap": provisional_cap, "result": result} {
  ns := quotas.tenants[input.cluster].namespaces[input.namespace]
  provisional_cap := input.requested_cap + ns.used_cap
  result := provisional_cap <= ns.total_cap
}
