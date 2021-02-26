Name:           karavi-authorization
Version:        0.1
Release:        0
Summary:        Karavi Authorization

License:        GPL
#URL:
%description
Install Karavi Authorization package

%install
mkdir -p $RPM_BUILD_ROOT%{_bindir}/karavi-authorization
echo "The dir is: "
pwd
cp ../../../bin/deploy $RPM_BUILD_ROOT/%{_bindir}/karavi-authorization/.

%files
%{_bindir}/karavi-authorization/deploy

%post
echo "Installing %{name}-%{version}.%{release}"
%{_bindir}/karavi-authorization/deploy
echo "Installation Complete"

%preun
/usr/local/bin/k3s-uninstall.sh
rm -rf /usr/local/bin/karavictl
rm -rf /var/lib/rancher/k3s/agent/images
rm -rf /var/lib/rancher/k3s/server/manifests

%postun
rm -rf %{_bindir}/karavi-authorization
