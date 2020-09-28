# Volume Ingestion
This script is for static provisioning in csi-powerscale. You can manage volumes which exist on PowerScale but were created outside of CSI driver.
## Prerequisites
The NFS export corresponding to the directory needs to exist. It's ID would be used in the volumehandle as described below. It need not have the kubernetes nodes added as clients. 

If Quotas are enabled in the driver, it is recommended that you add the Quota ID to the description of the NFS export in the following format:
CSI_QUOTA_ID:sC-kAAEAAAAAAAAAAAAAQEpVAAAAAAAA

The details of various Quotas can be obtained via the following REST API of OneFS:
GET 
/platform/1/quota/quotas

Volume Handle is expected to be present in this pattern VolName + VolumeIDSeparator + exportID + VolumeIDSeparator + accessZone for ex. "demovol1=\_=\_=303=\_=\_=System"

## Running the script
Command Line inputs are taken in this order: volumename, volumehandle, storageclassname, accessmode, storage size, pvname, pvcname 
## Examples
To provision a Volume named sample14 in access zone csi-zone having export-id as 6, volumehandle will be sample14=\_=\_=6=\_=\_=csi-zone. Here we are using a custom storage class named as customstorageclass1, access mode as ReadWriteMany, storage size as 500M, pv name as pv1, pvc name as pvc1, we will be running :

./ingestion_test.sh sample14 sample14=\_=\_=6=\_=\_=csi-zone customstorageclass1 ReadWriteMany 500M pv1 pvc1
