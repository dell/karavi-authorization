if getenforce | grep -q 'Enforcing'; then
    set -e
    [ -r /etc/os-release ] && . /etc/os-release
    if [ "${ID_LIKE%%[ ]*}" = "suse" ]; then
        os_env="microos"
        package_installer=zypper
    elif [ "${VERSION_ID%%.*}" = "7" ]; then
        os_env="centos7"
        package_installer=yum
    else
        os_env="centos8"
        package_installer=yum
    fi

    if [ "${package_installer}" = "yum" ] && [ -x /usr/bin/dnf ]; then
        package_installer=dnf
    fi

    echo "Installing K3s SELinux..."
    ${package_installer} install -y ${os_env}-k3s-selinux.rpm
    echo "K3s SELinux Installation Complete!"
else
    echo "SELinux is not enabled. Skipping installation of k3s-SELinux"
fi

set -e
rpm -ivh karavi-authorization-1.4-0.x86_64.rpm