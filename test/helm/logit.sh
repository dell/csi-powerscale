#!/bin/sh
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

