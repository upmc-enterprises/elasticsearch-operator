#!/bin/bash

# Copyright 2017 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

HACK_DIR=$(dirname "${BASH_SOURCE}")
REPO_ROOT=${HACK_DIR}/..

${REPO_ROOT}/vendor/k8s.io/code-generator/generate-groups.sh \
  all \
  github.com/upmc-enterprises/elasticsearch-operator/pkg/client \
  github.com/upmc-enterprises/elasticsearch-operator/pkg/apis \
  elasticsearchoperator:v1  \
  $@
