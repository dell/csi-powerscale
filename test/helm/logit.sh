#!/bin/sh
while true
do
date
date >>log.output
running=$(kubectl get pods -n test | grep "Running" | wc -l)
creating=$(kubectl get pods -n test | grep "ContainerCreating" | wc -l)
pvcs=$(kubectl get pvc -n test | wc -l)
echo running $running creating $creating pvcs $pvcs
echo running $running creating $creating pvcs $pvcs >>log.output
sleep 30
done

