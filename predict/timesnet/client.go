package timesnet

import (
	"time"
)

// python server 不在 k8s 部署，因此可以硬编码配置。
var (
	host     = "localhost"
	port     = 5000
	protocol = "http"
	timeout  = 5 * time.Second
)

func predict() {
	path := "/predict"

}
