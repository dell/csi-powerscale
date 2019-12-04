#!/bin/sh
username=$(cat secret.yaml | awk '/username:/{ print $2;}' | base64 -d)
echo username: $username
password=$(cat secret.yaml | awk '/password:/{ print $2;}' | base64 -d)
echo password: $password
