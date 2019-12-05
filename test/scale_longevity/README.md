# Helm charts and scripts for scalability and longevity tests

This test holds a few scripts that can be used to test scalability and longevity
for CSI driver

## Scalability
The scaletest.sh script can be used to kick off scalability tests. 
This will run one series of 3 helm deployments, causing a number of pods and replicas to be
created, each with a certain number of volumes. Once the specified number of replicas have
been launched, the deployments will be scaled down and removed.

Log files will be created in the directory, titled `scalability-YYYYMMDD-HHMMSS.log`

To launch the scalability test, a number of parameters are needed:

```
scaletest.sh
    -n namespace      - Namespace in which to place the test.
    -r replicas       - Number of replicas to create
    -v number_volumes - Number of volumes for each pod. Default is: 1

Default values for everything are set in the driver specific file at env.sh
```

An example of running the scalability tests for Isilon is:
```
./scaletest.sh -n test -r 33 -v 30
```
That test will deploy 3 helm charts, with 33 replicas of each pod and 30 volumes per pod (a total of 99 pods and 2970 volumes). Once the pods are running, the system will scale down and terminate the test.

## Longevity
The longevity.sh script can be used to kick off longevity tests. 
This will run a series of 3 helm deployments over and over, causing a number of pods and replicas to be created and deleted until the test is stopped. To stop the test, create a file named 'stop' in the directory. Upon stopping, the deployments will be scaled down and removed.

Log files will be created in the directory, titled `longevity-YYYYMMDD-HHMMSS.log`

To launch the longevity test, a number of parameters are needed:

```
longevity.sh
    -n namespace      - Namespace in which to place the test.
    -r replicas       - Number of replicas to create
    -v number_volumes - Number of volumes for each pod. Default is: 1

Default values for everything are set in the driver specific file at env.sh
```

An example of running the longevity tests for Isilon is:
```
./longevity.sh -n test -r 10 -v 10
```
That test will deploy 3 helm charts, with 10 replicas of each pod and 10 volumes per pod (a total of 30 pods and 300 volumes). Once the pods are running, the system will scale down to zero and repeat the process until stopped.

Note that values specified on the command line override values defined in the env file.