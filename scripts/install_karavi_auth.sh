#!/bin/bash
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

function usage() {
  echo
  echo "Help for $0"
  echo
  echo "Usage: $0 mode options..."
  echo "Mode:"
  echo -e "\t--upgrade \t\t\t\t\t\t\t\t Upgrades CSM Authorization when CSM Authorization is already installed"

  echo
  echo -e "\tOptional"
  echo ""
  echo -e "\t--traefik_web_port web_port --traefik_websecure_port websecure_port \t Sets traefik Nodeport web and websecure"
  echo ""
  echo -e "\tExample: $0 --traefik_web_port 30001 --traefik_websecure_port 30002"
  echo ""
  echo -e "\t--help \t\t\t\t\t\t\t\t\t Help"
  echo
  
  exit 0
}

UPGRADE=0
RPM_VERSION=1.5-0

while getopts ":h-:" optchar; do
  case "${optchar}" in
  -)
    case "${OPTARG}" in
    upgrade)
      UPGRADE=1
      ;;
    traefik_web_port)  val="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ))
        #echo "Parsing option: '--${OPTARG}', value: '${val}'" >&2;
        webPort=${val}
        ;;
    traefik_websecure_port) val="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ))
        #echo "Parsing option: '--${OPTARG}', value: '${val}'" >&2;
        websecurePort=${val}
        ;;
    help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option --${OPTARG}"
      echo "For help, run $PROG -h"
      exit 1
      ;;
    esac
    ;;
  *)
    echo "Unknown option -${OPTARG}"
    echo "For help, run $PROG -h"
    exit 1
    ;;
  esac
done

if [ $UPGRADE == 1 ]; then
    rpm -Uvh karavi-authorization-${RPM_VERSION}.x86_64.rpm --nopreun --nopostun
else
    if getenforce | grep -q 'Enforcing\|Permissive'; then
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
        echo "K3s SELinux Installation Complete"
    else
        echo "SELinux is not enabled. Skipping installation of k3s-SELinux"
    fi

    set -e
    rpm -ivh karavi-authorization-${RPM_VERSION}.x86_64.rpm
fi

sh ./policies/policy-install.sh

K3S=/usr/local/bin/k3s

if [ ! -z "$webPort" ] && [ ! -z "$websecurePort" ]
then
  while [ $($K3S kubectl get svc -n kube-system | grep traefik | wc -l) -ne 1 ]
  do
        echo "Waiting for traefik service to be available ..."
        sleep 10s
  done

  if [ $($K3S kubectl get svc -n kube-system | grep traefik | wc -l) -eq 1 ]
  then
        sh ./traefik_nodeport.sh --traefik_web_port $webPort --traefik_websecure_port $websecurePort
  fi
fi

echo "Installation Complete!"
