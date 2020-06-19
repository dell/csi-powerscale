# Volume Ingestion
This script is for static provisioning in csi-isilon. You can manage volumes which exist on Isilon but were created outside of CSI driver.
## Prerequisites
Volume Name is expected to be present in this pattern VolName + VolumeIDSeparator + exportID + VolumeIDSeparator + accessZone for ex. demovol1=\_=\_=303=\_=\_=System
## Running the script
Command Line inputs are taken in this order: volumename, storageclassname, accessmode, storage size, pvname, pvcname 
## Examples
To provision a Volume named sample14 in access zone csi-zone having export-id as 6, volumename will be sample14=\_=\_=6=\_=\_=csi-zone. Here we are using a custom storage class named as customstorageclass1, access mode as ReadWriteMany, storage size as 500M, pv name as pv1, pvc name as pvc1, we will be running :

./ingestion_test.sh sample14=\_=\_=6=\_=\_=csi-zone customstorageclass1 ReadWriteMany 500M pv1 pvc1
