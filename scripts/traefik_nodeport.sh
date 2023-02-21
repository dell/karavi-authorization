#!/bin/bash
# Copyright (c) 2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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

helpFunction()
{
   echo ""
   echo "Usage: $0 --traefik_web_port web_port --traefik_websecure_port websecure_port"
   echo -e "\t--traefik_web_port Traefik Nodeport web static port"
   echo -e "\t--traefik_websecure_port Traefik Nodeport websecure static port"
   echo ""
   echo "Example: $0 --traefik_web_port 30001 --traefik_websecure_port 30002"
   echo ""
   exit 1 # Exit script after printing help
}


while getopts ":h-:" optchar; do
  case "${optchar}" in
  -)
    case "${OPTARG}" in
    traefik_web_port)  val="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ))
        #echo "Parsing option: '--${OPTARG}', value: '${val}'" >&2;
        webPort=${val}
        ;;
    traefik_websecure_port) val="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ))
        #echo "Parsing option: '--${OPTARG}', value: '${val}'" >&2;
        websecurePort=${val}
        ;;
    help)
      helpFunction
      exit 0
      ;;
    *)
      echo "Unknown option --${OPTARG}"
      echo "For help, run $PROG --help"
      exit 1
      ;;
    esac
    ;;
  *)
    echo "Unknown option -${OPTARG}"
    echo "For help, run $PROG --help"
    exit 1
    ;;
  esac
done

# Print helpFunction in case parameters are empty
if [ -z "$webPort" ] || [ -z "$websecurePort" ]
then
   echo "Some or all of the parameters are empty";
   helpFunction
fi

# Begin script in case all parameters are correct
echo "Setting Traefik Nodeport web to $webPort"
k3s kubectl patch svc/traefik -n kube-system --type='json' -p='[{"op": "replace", "path": "/spec/ports/0/nodePort", "value":'$webPort'}]'
echo "Setting Traefik Nodeport websecure to $websecurePort"
k3s kubectl patch svc/traefik -n kube-system --type='json' -p='[{"op": "replace", "path": "/spec/ports/1/nodePort", "value":'$websecurePort'}]'