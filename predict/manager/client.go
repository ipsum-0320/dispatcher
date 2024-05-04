package manager

import "predict/timesnet"

// manager 通过 k8s 部署，因此最好使用 configmap 获取 manager 的相关配置。
var (
	managerIP   string
	managerPort int32
)

func init() {
	// 从配置文件中读取 manager 的相关配置

}

func Manage(predResponse *timesnet.PredDataResponse) {

}
