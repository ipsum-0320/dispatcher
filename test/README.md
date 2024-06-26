# 测试流程

每次将新功能加入之后，都需要进行测试，以确保新功能的正确性。

shell目录下有reset.sh和start.sh两个脚本文件

reset.sh，用于恢复到原始的测试环境
步骤如下：
1. 停止fakeuser、predict和usercenter模块
2. 重置数据库，包括修改instance_huadong表中实例状态为available、重置record_huadong表、清空bounce_huadong表
3. 调用manager模块的接口，missing设置为0，实现释放K8S集群中所有弹性实例
4. 调用DisconnectAllInstances程序，实现将K8S集群中所有实例断开连接(程序由main.go构建DisconnectAllInstances可执行程序)
5. 停止manager模块


start.sh，用于打包镜像，重启系统
如下步骤：
1. 根据参数判断是否需要重新打包镜像
2. 重启manager、usercenter模块
3. 等待manager模块部署完成
4. 重启predict模块
5. 等待predict模块部署完成，并完成一次预测
6. 部署fakeuser模块


使用方法：
每次测试之后，需要执行reset脚本恢复测试环境
如果想要重新测试，需要执行start脚本，可以附带参数，如果为true，则会自动重新打包镜像

注意：
如果在reset的时候，正好manager正在申请弹性实例，可能会拉起实例但是没有保存到数据库，
所以每次执行reset之后，最好还是去K8S集群上看一下是否有弹性实例，把它以及数据库中的数据删除掉

测试 save rate 的接口，请使用 apifox 等工具，因为其会对空格 query 进行转译。

![img.png](img/apifox.png)

连接 cloudgame 数据库如下:
```bash
mysql -h 10.10.103.51 -P 30565 -u root -p

use cloudgame;

show tables;
```