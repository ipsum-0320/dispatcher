package manager

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"predict/timesnet"
	"runtime"
)

// manager 通过 k8s 部署，因此最好使用 configmap 获取 manager 的相关配置。
var (
	protocol    = "http"
	managerIP   string
	managerPort int32
)

type Config struct {
	ip   string
	port int32
}

func init() {
	// 从配置文件中读取 manager 的相关配置。
	_, filename, _, _ := runtime.Caller(0)
	curDir := filepath.Dir(filename)
	managerConfigPath := filepath.Join(curDir, "manager.yaml")
	yamlFile, err := os.ReadFile(managerConfigPath)
	if err != nil {
		fmt.Printf("Failed to read YAML: %v", err)
		return
	}
	var managerConfig Config
	if err := yaml.Unmarshal(yamlFile, &managerConfig); err != nil {
		fmt.Printf("Failed to unmarshal YAML: %v", err)
		return
	}

	managerIP = managerConfig.ip
	managerPort = managerConfig.port
}

func Calc(predResponse *timesnet.PredDataResponse, siteId string) (int32, error) {
	// TODO: 查询目前当前边缘站点可以容纳多少实例（在不申请弹性资源的情况下）。

	// TODO: 计算将要释放或者申请的实例数目。
	return 0, nil
}

func Manage(zoneId string, replica int32) error {
	// TODO: 请求 manager 申请或者释放实例。
	return nil
}

type DeploymentApplyRequest struct {
	DeploymentName string
	SiteId         string
	Replica        int32
}

func apply() {
	//path := "deployment/apply"
	//url := fmt.Sprintf("%s://%s:%d%s", protocol, managerIP, managerPort, path)

}

func podWatch() {

}
