# Various test Helm charts and test scripts

## Helm charts

| Name    | Usage |
|---------|-------|
|2vols    | Creates 2 filesystem mounts  |
|7vols    | Creates 7 filesystem mounts  |
|10vols   | Creates 10 filesystem mounts |

## Scripts

| Name           | Usage |
|----------------|-------|
| deletepvcs.sh  | Script to delete all PVCS in a namespace |
| get.volume.ids | Script to list the volume IDs for all PVS |
| logit.sh       | Script to print number of pods and pvcs in a namespace |
| starttest.sh   | Used to instantiate one of the Helm charts above. Requires argument of Helm chart |
| stoptest.sh    | Stops currently running Helm chart and deletes all PVCS |
| snaptest.sh    | Script to create volume and snapshot from volume |

## Usage

The starttest.sh script is used to deploy Helm charts that test the deployment of a simple pod
with various storage configurations. The stoptest.sh script will delete the Helm chart and cleanup after the test.
Procedure

1. Navigate to the test/helm directory, which contains the starttest.sh and various Helm charts.

2. Run the starttest.sh script with an argument of the specific Helm chart to deploy and test. For example:

    > ./starttest.sh -t 2vols

3. After the test has completed, run the stoptest.sh script to delete the Helm chart and cleanup the volumes.

    > ./stoptest.sh -t 2vols
