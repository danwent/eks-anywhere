#!/usr/bin/env bash
# Copyright 2020 Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.


# The generate-attribution script requires a GIT_TAG file in the project
# root. The CLI repo doesn't have such a file, so just generating a temporary
# one from the head commit hash
function build::create_git_tag(){
    git rev-parse HEAD > GIT_TAG
}

# The api/v1alpha1 package of the release submodule in the CLI repo does not have 
# a license file. Hence we need to copy it into the vendor directory for go-licenses
# to pick up
function build::fix_licenses(){
    cp LICENSE ./vendor/github.com/aws/eks-anywhere/release/api/v1alpha1/LICENSE
}

# We only need to include dependencies outside of this repo
function build::exclude_own(){
    sed -i '/^github.com\/aws\/eks-anywhere/d' _output/attribution/go-license.csv
}
