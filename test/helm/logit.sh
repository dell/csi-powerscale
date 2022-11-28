#!/bin/sh
# Copyright © 2019-2021 Dell Inc. or its subsidiaries. All Rights Reserved.
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

while true
do
date
date >>log.output
running=$(kubectl get pods -n test --no-headers=true | grep "Running" | wc -l)
creating=$(kubectl get pods -n test --no-headers=true | grep "ContainerCreating" | wc -l)
pvcs=$(kubectl get pvc -n test --no-headers=true | wc -l)
echo pods running $running, pods creating $creating, pvcs count $pvcs
echo pods running $running, pods creating $creating, pvcs count $pvcs >>log.output
sleep 30
done

