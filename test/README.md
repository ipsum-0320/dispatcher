# 测试流程

每次将新功能加入之后，都需要进行测试，以确保新功能的正确性。

恢复到原始的测试环境包括如下步骤：

1. 要把K8S集群和数据库里的弹性实例都删掉，调用接口 manager 接口即可，在将 manager 拉起来的情况下，运行下面的脚本。
```bash
curl -X POST http://localhost:31365/instance/manage -d '{"zone_id":"huadong","missing":0}'
```
2. 向集群中的所有实例调用disconnect接口，断开连接，首先需要将本地的 server.go 拉起来，然后在命令行执行。
```bash
curl -X GET http://localhost:9988/disconnectAll
```
3. record数据表清空，然后插入开始时间之前的180条数据。
```sql
delete from record_huadong;

insert into record_huadong select * from record_huadong_backup where date >= "2024-05-09 09:00:00" and date < "2024-05-09 12:00:00";
```
4. 将bounce数据表清空。
```sql
delete from bounce_huadong;
```

再之后，重新打包相关的镜像，以 predict 和 manager 为例，可有如下 bash 脚本:
```bash
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
```

测试 save rate 的接口：
```bash
curl -X GET "http://localhost:31365/bounce/rate" \
--data-urlencode "start=2024-05-09 12:00:00" \
--data-urlencode "end=2024-05-09 12:10:00"
```