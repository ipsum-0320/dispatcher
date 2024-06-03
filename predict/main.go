package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"predict/config"
	"predict/process"
	"syscall"
	"time"

	mysql_service "predict/mysql/service"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	// 从业务集群中获取 config，并创建客户端。
	conf, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Error creating Kubernetes config: %v", err)
	}
	client, err := kubernetes.NewForConfig(conf)
	if err != nil {
		log.Fatalf("Error creating Kubernetes client: %v", err)
	}

	zoneList, err := mysql_service.GetZoneListInDB()
	if err != nil {
		log.Fatalf("Error getting zoneList in database: %v", err)
	}

	run := func(ctx context.Context) {
		go func() {
			// 确立心跳检测服务。
			http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
				_, err := fmt.Fprintf(w, "Alive")
				if err != nil {
					log.Fatalf("error writing response: %v", err)
				}
			})
			if err := http.ListenAndServe(fmt.Sprintf("0.0.0.0:%s", config.PREDICTPORT), nil); err != nil {
				fmt.Println("server serve failed:", err)
			}
		}()

		// 创建一个定时任务，每隔 15 分钟执行一次。
		wait.Until(func() {
			for zoneId, siteList := range zoneList {
				go func(zoneId string, siteList []string) {
					err := process.Process(zoneId, siteList)
					if err != nil {
						fmt.Printf("%s process failed, err:%v\n", zoneId, err)
						return
					}
				}(zoneId, siteList)
			}
		}, time.Duration(15*60*1000/config.ACCELERATIONRATIO)*time.Millisecond, ctx.Done())
	}

	// 创建分布式锁。
	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		config.K8SNAMSPACE,
		"predict-lock",
		client.CoreV1(),
		client.CoordinationV1(),
		resourcelock.ResourceLockConfig{
			Identity: string(uuid.NewUUID()),
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
