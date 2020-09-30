#!/bin/bash

docker run -p 8181:8181 --rm -d openpolicyagent/opa \
	run --server --log-level debug

sleep 5

curl -v -X PUT --data-binary @opa_data.json \
	http://localhost:8181/v1/data/dell/quotas

curl -v -X PUT --data-binary @opa_create_volume.rego \
	http://localhost:8181/v1/policies/dell
