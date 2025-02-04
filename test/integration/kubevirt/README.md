# KubeVirt Qual files for CSI PowerScale driver for OpenShift virtualization CSI certification tests

Check https://confluence.cec.lab.emc.com/display/CSIECO/KubeVirt for details on running KubeVirt tests.

manifest.yaml - use this file as an input to the Kubevirt test.
isilon-sc.yaml - This storage class yaml is referred to by the manifest and used by the test. Ensure the Directory already exists on the array, or rename it.
