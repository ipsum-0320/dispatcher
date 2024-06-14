#!/bin/bash

# 1. repackage images with params
if [ "$1" == "true" ]; then
    # 1.1 remove old images
    docker rmi dispatcher-manager dispatcher-predict dispatcher-usercenter dispatcher-fakeuser

    # 1.2 package new images
    cd ~/dispatcher/manager/
    rm -rf manager
    go build -o manager main.go
    docker build -t dispatcher-manager .

    cd ~/dispatcher/predict/
    rm -rf predict
    go build -o predict main.go
    docker build -t dispatcher-predict .

    cd ~/dispatcher/usercenter/
    rm -rf usercenter
    go build -o usercenter main.go
    docker build -t dispatcher-usercenter .

    cd ~/dispatcher/fakeuser/
    rm -rf fakeuser
    go build -o fakeuser main.go
    docker build -t dispatcher-fakeuser .
fi

# 2. restart manager, usercenter
cd ~/dispatcher/deploys/deployments/
kubectl apply -f manager.yaml
kubectl apply -f usercenter.yaml
sleep 30

# 3. restart predict
kubectl apply -f predict.yaml
sleep 30

# 4. restart fakeuser
kubectl apply -f fakeuser.yaml
