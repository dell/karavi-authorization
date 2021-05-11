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

package karavi.authz.powermax.url

allowlist = [
	"GET /univmax/restapi/version",
	"GET /univmax/restapi/(90|91)/system/symmetrix/[a-f0-9]+",
	"GET /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/srp",
	"GET /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/storagegroup",
	"POST /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/storagegroup",
	"GET /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/storagegroup/[a-f0-9]+",
	"PUT /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/storagegroup/[a-f0-9]+",
	"GET /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/volume",
	"GET /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/volume/[a-f0-9]+",
	"PUT /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/volume/[a-f0-9]+",
	"DELETE /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/volume/[a-f0-9]+",
	"DELETE /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/storagegroup/[a-f0-9]+",
	"GET /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/volume/[a-f0-9]+/snapshot",
	"GET /univmax/restapi/91/sloprovisioning/symmetrix/[a-f0-9]+/portgroup/(.+)",
	"GET /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/initiator",
	"GET /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/host/(.+)",
	"GET /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/maskingview/(.+)",
	"GET /univmax/restapi/(90|91)/system/symmetrix",
	"GET /univmax/restapi/private/(90|91)/replication/symmetrix/[a-f0-9]+/volume/[a-f0-9]+/snapshot",
	"GET /univmax/restapi/private/(90|91)/replication/symmetrix/[a-f0-9]+/volume/",
	"DELETE /univmax/restapi/(90|91)/sloprovisioning/symmetrix/[a-f0-9]+/maskingview/(.+)",
]

default allow = false

allow {
	regex.match(allowlist[_], sprintf("%s %s", [input.method, input.url]))
}
