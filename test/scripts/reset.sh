#!/bin/bash

pre_record="2024-05-15 21:00:00"
start_time="2024-05-16 00:00:00"
scale_ratio="6"
acceleration_ratio="5"

# 0. edit test-config 
# 使用kubectl patch命令修改ConfigMap
kubectl patch configmap test-config \
  -p="{\"data\":{\"START_TIME\":\"${start_time}\", 
                 \"ACCELERATION_RATIO\":\"${acceleration_ratio}\",
                 \"SCALE_RATIO\":\"${scale_ratio}\"}}"

# 1. stop fakeuser, predict and usercenter module
cd ~/cloudgame/deploys/deployments/
kubectl delete -f fakeuser.yaml
kubectl delete -f predict.yaml
kubectl delete -f usercenter.yaml
sleep 5

# 2 reset database

# 2.1 reset instances' status
kubectl exec -it mysql -- mysql -uroot -p'cloudgame' -e "use cloudgame; update instance_huadong set status = 'available', device_id = 'null' where status = 'using';"

# 2.2 reset record_huadong table
kubectl exec -it mysql -- mysql -uroot -p'cloudgame' -e "use cloudgame; delete from record_huadong; insert into record_huadong (site_id, date, instances) select site_id, date, instances / ${scale_ratio} from history_huadong where date >= '${pre_record}' and date < '${start_time}';"

# 2.3 reset bounce_huadong table
kubectl exec -it mysql -- mysql -uroot -p'cloudgame' -e "use cloudgame; delete from bounce_huadong;"

# 3. disconnect all instances
cd ~/cloudgame/dispatcher/test/
go run main.go

# 4. call manager api to release elastic instances
curl -X POST http://localhost:31365/instance/manage -d '{"zone_id":"huadong","missing":0}'

# 5. stop manager
cd ~/cloudgame/deploys/deployments/
kubectl delete -f manager.yaml

