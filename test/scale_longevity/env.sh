#!/bin/bash

# Copyright © 2020-2025 Dell Inc. or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#

#!/bin/bash
# Copyright © 2019 Dell Inc. or its subsidiaries. All Rights Reserved.
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


# Configuration file for isilon driver
# This file is sourced from the longevity and scalbility test scripts
# It will be executed with 'cwd' set to this directory

# Longevity test helm chart
LONGTEST="${LONGTEST:-volumes}"
LONGREPLICAS="${LONGREPLICAS:-2}"

# Scalability test helm chart
SCALETEST="${SCALETEST:-volumes}"
SCALEREPLICAS="${SCALEREPLICAS:-3}"

# Namespace to run tests in. Will be created if it does not exist
NAMESPACE="${NAMESPACE:-test}"

# Storage Class names. Three of these are required but they can be the same.
# These must exist before starting tests
STORAGECLASS1="${STORAGECLASS1:-isilon}"
STORAGECLASS2="${STORAGECLASS2:-isilon}"
STORAGECLASS3="${STORAGECLASS3:-isilon}"

# The namespace that the driver is running within
DRIVERNAMESPACE="isilon"