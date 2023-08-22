#!/bin/bash

# Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License

# overrides file
# this file, included from the Makefile, will overlay default values with environment variables
#

microcontainer=$(buildah from $1)
micromount=$(buildah mount $microcontainer)
dnf install --installroot $micromount --releasever=9 --nodocs --setopt install_weak_deps=false --setopt=reposdir=/etc/yum.repos.d/ libaio libuuid numactl xfsprogs e4fsprogs nfs-utils -y
dnf clean all --installroot $micromount
buildah umount $microcontainer
buildah commit $microcontainer csipowerscale-ubimicro
