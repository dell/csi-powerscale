#!/bin/bash
# Copyright: (c) 2020, Dell EMC

# volumename  volumehandle  storageclass  accessmode  storagesize   pvname   pvcname
#     1             2            3             4            5         6         7

if [[ $2 =~ ^[a-zA-Z0-9_-]+=_=_=[0-9]+=_=_=[a-zA-Z0-9_-]+$ ]]
then
  echo "Volume handle pattern matched"

  #getting ip from storageclass:
  ip=$(kubectl get storageclass $3 -o yaml| grep AzServiceIP: | tail -1 | cut -d ':' -f 2,2| sed -e 's/^[[:space:]]*//' )

  #getting path from storageclass
  pathnew=$(kubectl get storageclass $3 -o yaml| grep IsiPath: | tail -1 | cut -d ':' -f 2,2| sed -e 's/^[[:space:]]*//' )
  path="$pathnew/$1"

  #getting accesszone from storage class
  acz=$(kubectl get storageclass $3 -o yaml| grep AccessZone: |cut -d ':' -f 2,2| sed -e 's/^[[:space:]]*//' )

   cat <<EOF > static_pv_$6.yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: $6
  namespace: default
spec:
  capacity:
    storage: "$5"
  accessModes:
    - "$4"
  persistentVolumeReclaimPolicy: Retain
  storageClassName: $3
  csi:
    driver: csi-isilon.dellemc.com
    volumeAttributes:
        Path: "$path"
        Name: "$1"
        AzServiceIP: '$ip'
    volumeHandle: $2
  claimRef:
    name: $7
    namespace: default
EOF

  cat <<EOF > static_pvc_$7.yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: $7
  namespace: default
spec:
  accessModes:
  - "$4"
  resources:
    requests:
      storage: "$5"
  volumeName: "$6"
EOF

  cat <<EOF > static_pod.yaml
apiVersion: v1
kind: Pod
metadata:
  name: testpod-script
  namespace: default
spec:
  containers:
    - name: task-pv-containe
      image: nginx
      ports:
        - containerPort: 80
          name: "http-server"
      volumeMounts:
        - mountPath: "/usr/share/nginx/html"
          name: nov-eleventh-1-pv-storage
  volumes:
    - name: nov-eleventh-1-pv-storage
      persistentVolumeClaim:
        claimName: $7
EOF

  kubectl create -f static_pv_$6.yaml
  echo "Persistent Volume $6 creation successful"
  kubectl create -f static_pvc_$7.yaml
  echo "Persistent Volume Claim $7 creation successful"
  kubectl create -f static_pod.yaml
  echo "Pod created successfully with ingested volume"

  #cleaning files created:
  rm -rf static_pv_$6.yaml static_pvc_$7.yaml static_pod.yaml

else
  echo "Volume handle $2 pattern does not matched the regex: ^[a-zA-Z0-9_-]+=_=_=+[0-9]*=_=_=+[a-zA-Z0-9_-]+$"
  echo "Volume handle is expected in this format: demovol=_=_=251=_=_=System"

fi
echo "done"




