# Copyright © 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
FROM registry.access.redhat.com/ubi9-micro:9.2-9
LABEL vendor="Dell Inc." \
      name="csm-authorization" \
      summary="Dell Container Storage Modules (CSM) for Authorization" \
      description="CSM for Authorization provides storage and Kubernetes administrators the ability to apply RBAC for Dell CSI Drivers" \
      version="2.0.0" \
      license="Apache-2.0"
ARG APP

WORKDIR /app
COPY $APP /app/command

ENTRYPOINT [ "/app/command" ]
