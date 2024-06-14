#!/bin/bash

# 1. stop fakeuser, predict and usercenter module
cd ~/dispatcher/deploys/deployments/
kubectl delete -f fakeuser.yaml
kubectl delete -f predict.yaml
kubectl delete -f usercenter.yaml
sleep 5

# 2 reset database
kubectl exec -it mysql -- mysql -uroot -pcloudgame -e "use cloudgame; update instance_huadong set status = 'available', device_id = 'null' where status = 'using'; delete from record_huadong; insert into record_huadong select * from record_huadong_backup where date >= '2024-05-10 21:00:00' and date < '2024-05-11 00:00:00'; delete from bounce_huadong;"

# 3. call manager api to release elastic instances
curl -X POST http://localhost:31365/instance/manage -d '{"zone_id":"huadong","missing":0}'

# 4. disconnect all instances
cd ~/dispatcher/test/
./DisconnectAllInstances

# 5. stop manager
cd ~/dispatcher/deploys/deployments/
kubectl delete -f manager.yaml

