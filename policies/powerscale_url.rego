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

package karavi.authz.url

allowlist = [
    "GET /namespace/.+",
    # GET /platform/2/protocols/nfs/exports/?path=%2Fifs%2Faaron%2Faaron-k8s-98517ad9b0&zone=System
    "GET /platform/[0-9]/protocols/nfs/exports/",
    "PUT /namespace/.+",
    "GET /platform/[0-9]/quota/license/",
    "POST /platform/[0-9]/quota/quotas/",
    # POST /platform/2/protocols/nfs/exports/?zone=System
    "POST /platform/[0-9]/protocols/nfs/exports/",
    # GET /platform/2/protocols/nfs/exports/67485?zone=System
    "GET /platform/[0-9]/protocols/nfs/exports/[0-9]+",
    # PUT /platform/2/protocols/nfs/exports/67485?zone=System
    "PUT /platform/[0-9]/protocols/nfs/exports/[0-9]+",
    "DELETE /platform/[0-9]/quota/quotas/[a-f0-9A-F]+"
    # DELETE /platform/2/protocols/nfs/exports/67485?zone=System
    "DELETE /platform/[0-9]/protocols/nfs/exports/[0-9]+"
    # DELETE /namespace/ifs/csm-aaron/aaron-k8s-199d622edf?recursive=true
    "DELETE /namespace/.+"
    "POST /proxy/refresh-token/"
]

default allow = false
allow {
	regex.match(allowlist[_], sprintf("%s %s", [input.method, input.url]))
}
