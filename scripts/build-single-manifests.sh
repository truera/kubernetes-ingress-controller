#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(dirname ${BASH_SOURCE})/..

cd $REPO_ROOT

${REPO_ROOT}/bin/kustomize build config/base > deploy/single/all-in-one-dbless-legacy.yaml
${REPO_ROOT}/bin/kustomize build config/variants/postgres > deploy/single/all-in-one-postgres.yaml
${REPO_ROOT}/bin/kustomize build config/variants/enterprise > deploy/single/all-in-one-dbless-k4k8s-enterprise.yaml
${REPO_ROOT}/bin/kustomize build config/variants/enterprise-postgres > deploy/single/all-in-one-postgres-enterprise.yaml
${REPO_ROOT}/bin/kustomize build config/variants/multi-gw/oss > deploy/single/all-in-one-dbless.yaml
${REPO_ROOT}/bin/kustomize build config/variants/multi-gw/enterprise > deploy/single/all-in-one-dbless-enterprise.yaml
${REPO_ROOT}/bin/kustomize build config/variants/konnect/oss > deploy/single/all-in-one-dbless-konnect.yaml
${REPO_ROOT}/bin/kustomize build config/variants/konnect/enterprise > deploy/single/all-in-one-dbless-konnect-enterprise.yaml
