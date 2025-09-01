/*
 *
 * Copyright © 2021-2024 Dell Inc. or its subsidiaries. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// K8sValueFile values file that stories k8s node labels applied
const K8sValueFile = "k8sValues.csv"

// K8sLabel defines the field label to be used
const K8sLabel = "label"

// WriteK8sValueToFile writes the values to a file to used in fakeNode
func WriteK8sValueToFile(inputType, value string) {
	DeleteK8sValuesFile()
	log.Printf("writing k8s values input=%s, value=%s", inputType, value)
	pwd, _ := os.Getwd()
	log.Printf("cwd is path is %s", pwd)
	file, err := os.Create(K8sValueFile)
	if err != nil {
		log.Printf("Unable to create file to write kubernetes values - %s", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Error closing file: %s\n", err)
		}
	}()
	valuesCsvStr := fmt.Sprintf("%s,%s", inputType, value)
	log.Printf("values str is %s", valuesCsvStr)
	_, err = file.WriteString(valuesCsvStr)
	if err != nil {
		log.Printf("Unable to create file to write kubernetes values - %s", err)
	}
	absPath, err := filepath.Abs(file.Name())
	if err != nil {
		log.Printf("Unable to create abspath of file  - %s", err)
	}
	fmt.Printf("wrote the values to file - %s \n", absPath)
}

// DeleteK8sValuesFile deletes the values in the file used in fakeNode
func DeleteK8sValuesFile() bool {
	abspath, err := filepath.Abs(K8sValueFile)
	if err != nil {
		log.Errorf("unable get abs path of file for deletion - %s", err)
	}
	log.Infof("abs path for deletion is %s", abspath)
	pwd, _ := os.Getwd()
	log.Infof("cwd for deletion is %s", pwd)
	err = os.Remove(K8sValueFile)
	if err != nil {
		log.Errorf("unable to remove file %s - %s", K8sValueFile, err)
		return false
	}
	log.Infof("deleted file %s", abspath)
	return true
}

func readAppliedLabels() (string, string) {
	var label string
	var value string
	abspath, err := filepath.Abs(K8sValueFile)
	if err != nil {
		log.Errorf("unable to get abs path of filei to read due to - %s", err)
	}
	log.Infof("abs path to read is is %s", abspath)
	pwd, _ := os.Getwd()
	log.Infof("cwd while reading is path is %s", pwd)
	content, err := os.ReadFile(K8sValueFile)
	if err != nil {
		log.Errorf("unable to read file - %s", err)
		return label, value
	}
	valueStr := string(content)
	log.Infof("content read is %s", valueStr)
	values := strings.Split(valueStr, ",")
	if len(values) > 0 {
		labelnValues := strings.Split(values[1], "=")
		if len(labelnValues) > 1 {
			label = labelnValues[0]
			value = labelnValues[1]
		}
	}
	log.Printf("sent label values %s - %s ", label, value)
	return label, value
}

// GetFakeNode returns a fake node with labels including current hostname
func GetFakeNode() *v1.Node {
	client := fake.NewSimpleClientset()
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "fake-host"
		log.Errorf("setting hostname as %s as call to get hostname failed", hostname)
	}
	log.Print(hostname)
	labelMap := make(map[string]string)
	labelMap["beta.kubernetes.io/arch"] = "amd64"
	labelMap["beta.kubernetes.io/os"] = "linux"
	labelMap["kubernetes.io/arch"] = "amd64"
	labelMap["kubernetes.io/hostname"] = hostname
	labelMap["kubernetes.io/os"] = "linux"
	label, value := readAppliedLabels()
	if label != "" {
		fmt.Printf("wrote %s=%s\n", label, value)
		labelStr := label
		val := "" + value + ""
		labelMap[labelStr] = val
	}
	labelMap["csi-isilon.dellemc.com/127.0.0.1"] = "csi-isilon.dellemc.com"

	node := &v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:                       "test-node",
			UID:                        "apple123-6a3b-4e6c-9fc3-a8b9f3ecf8ec",
			ResourceVersion:            "12628879",
			Generation:                 0,
			CreationTimestamp:          metav1.Time{},
			DeletionTimestamp:          nil,
			DeletionGracePeriodSeconds: nil,
			Labels:                     labelMap,
			Annotations:                nil,
			OwnerReferences:            nil,
			Finalizers:                 nil,
			// ClusterName:                "",
			ManagedFields: nil,
		},
		Spec:   v1.NodeSpec{},
		Status: v1.NodeStatus{},
	}

	fakeNode, err := client.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
	if err != nil {
		log.Errorf("Error occured while creating pod %s: %s", node.Name, err.Error())
	}
	return fakeNode
}
