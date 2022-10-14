:'
 Copyright © 2021 Dell Inc. or its subsidiaries. All Rights Reserved.
 
 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
'
#!/bin/bash

docker run -p 8181:8181 --rm -d openpolicyagent/opa \
	run --server --log-level debug

sleep 5

curl -v -X PUT --data-binary @opa_data.json \
	http://localhost:8181/v1/data/dell/quotas

curl -v -X PUT --data-binary @opa_create_volume.rego \
	http://localhost:8181/v1/policies/dell
