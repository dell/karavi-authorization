package karavi.authz.url

allowlist = [
		"POST /proxy/refresh-token/",
		"GET /api/version/",
		"GET /api/types/System/instances/",
		"GET /api/types/StoragePool/instances/",
		"POST /api/types/Volume/instances/",
		"GET /api/instances/Volume::[a-f0-9]+/$",
		"POST /api/types/Volume/instances/action/queryIdByKey/",
		"GET /api/instances/System::[a-f0-9]+/relationships/Sdc/",
		"POST /api/instances/Volume::[a-f0-9]+/action/addMappedSdc/",
		"POST /api/instances/Volume::[a-f0-9]+/action/removeMappedSdc/",
		"POST /api/instances/Volume::[a-f0-9]+/action/removeVolume/"
]

default allow = false
allow {
	regex.match(allowlist[_], sprintf("%s %s", [input.method, input.url]))
}
