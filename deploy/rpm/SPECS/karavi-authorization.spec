# Copyright (c) 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
# http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

Name:           karavi-authorization
Version:        1.4
Release:        0
Summary:        Karavi Authorization

License:        ASL 2.0
#URL:
%description
Install Karavi Authorization package

%install
mkdir -p $RPM_BUILD_ROOT%{_tmppath}
cp deploy $RPM_BUILD_ROOT%{_tmppath}

%files
%{_tmppath}/deploy

%post
echo "Installing %{name}-%{version}.%{release}"
%{_tmppath}/deploy
echo "%{name}-%{version}.%{release} Installation Complete"

%preun
/usr/local/bin/k3s-uninstall.sh
rm -rf /usr/local/bin/karavictl
rm -rf /var/lib/rancher/k3s/agent/images
rm -rf /var/lib/rancher/k3s/server/manifests

%postun
rm -rf %{_tmppath}/deploy
echo "Uninstall Complete"
