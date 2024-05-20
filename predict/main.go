package main

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"log"
	"net/http"
	"os/signal"
	_ "predict/manager"
	_ "predict/mysql"
	"syscall"
	"time"
)

var (
	ns   = "default"
	host = "0.0.0.0"
	port = 8080
	// 由于分库分表的设计，zoneId 以表为单位单独列出，zoneId 的列表需要写死。
	zoneIds = []string{"huadong", "xibei"}
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	// 从业务集群中获取 config，并创建客户端。
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error creating Kubernetes config: %v", err)
	}
	c, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	id := string(uuid.NewUUID())

	run := func(ctx context.Context) {
		// 确立心跳检测服务。
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			_, err := fmt.Fprintf(w, "Alive")
			if err != nil {
				log.Fatalf("error writing response: %v", err)
			}
		})
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil); err != nil {
			fmt.Println("server serve failed:", err)
		}
		// 创建一个定时任务，每隔 15 分钟执行一次。
		wait.Until(func() {
			for _, zoneId := range zoneIds {
				go func(zoneId string) {
					err := Process(zoneId)
					if err != nil {
						fmt.Printf("%s process failed, err:%v\n", zoneId, err)
						return
					}
				}(zoneId)
			}
		}, 15*time.Minute, ctx.Done())
	}

	// 创建分布式锁。
	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		ns,
		"predict-lock",
		c.CoreV1(),
		c.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: id,
		})
	if err != nil {
		klog.Fatalf("error creating lock: %v", err)
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: 15 * time.Second,
		RenewDeadline: 10 * time.Second,
		RetryPeriod:   2 * time.Second,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				klog.Fatalf("leaderelection lost")
			},
		},
		WatchDog: nil,
		Name:     "predict",
	})
}
