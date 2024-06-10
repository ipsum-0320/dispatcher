cd ~/dispatcher/deploys/deployments/
kubectl delete -f fakeuser.yaml
kubectl delete -f predict.yaml
kubectl delete -f manager.yaml
docker rmi dispatcher-predict dispatcher-manager
cd ~/dispatcher/manager/
rm -rf manager
go build
docker build . -t dispatcher-manager
cd ~/dispatcher/predict/
rm -rf predict
go build
docker build . -t dispatcher-predict
cd ~/dispatcher/deploys/deployments/