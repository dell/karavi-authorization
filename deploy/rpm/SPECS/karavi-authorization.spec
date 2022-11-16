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
echo "RPM Installation Complete"

%preun
/usr/local/bin/k3s-uninstall.sh
rm -rf /usr/local/bin/karavictl
rm -rf /var/lib/rancher/k3s/agent/images
rm -rf /var/lib/rancher/k3s/server/manifests

%postun
rm -rf %{_tmppath}/deploy
echo "Uninstall Complete"
