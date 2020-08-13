#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail
go mod vendor

if [ -d "$GOPATH/src/k8s.io/code-generator" ]; then
  CODEGEN_PKG="$GOPATH/src/k8s.io/code-generator"
fi

SCRIPT_ROOT=`pwd`/$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/Soluto-Private/kubernetes-icinga/pkg/client github.com/Soluto-Private/kubernetes-icinga/pkg/apis \
  icinga.nexinto.com:v1 \
  --output-base "${SCRIPT_ROOT}"/../../../ \
  --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

find pkg/client -type f -name '*.go' -print0 | xargs -0 sed -i '' 's/github.com\/nexinto/github.com\/Nexinto/g'

