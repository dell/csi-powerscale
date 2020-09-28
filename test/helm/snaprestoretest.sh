#!/bin/sh
echo "installing a 2 volume container"
sh starttest.sh -t 2vols
echo "done installing a 2 volume container"

echo "marking volume"
kubectl exec -n test isilontest-0 -- touch /data0/orig
kubectl exec -n test isilontest-0 -- ls -l /data0
kubectl exec -n test isilontest-0 -- sync
kubectl exec -n test isilontest-0 -- sync

echo "creating snap1 of pvol0"
kubectl create -f snap1.yaml
sleep 20
kubectl get volumesnapshot -n test

echo "updating container to add a volume sourced from snapshot"
helm upgrade 2vols 2vols+restore
echo "waiting for container to upgrade/stabalize"
sleep 20
up=0
while [ $up -lt 1 ]; do
    sleep 5
    kubectl get pods -n test
    up=`kubectl get pods -n test | grep '1/1 *Running' | wc -l`
done
kubectl describe pods -n test
kubectl exec -n test isilontest-0 -it df | grep data
kubectl exec -n test isilontest-0 -it mount | grep data
echo "updating container finished"

echo "marking volume"
kubectl exec -n test isilontest-0 -- touch /data2/new
echo "listing /data0"
kubectl exec -n test isilontest-0 -- ls -l /data0
echo "listing /data2"
kubectl exec -n test isilontest-0 -- ls -l /data2
sleep 20

echo "deleting container"
sh stoptest.sh -t 2vols
sleep 5

echo "deleting snap"
kubectl delete volumesnapshot pvol0-snap1 -n test
