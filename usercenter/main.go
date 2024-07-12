package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os/signal"
	"sync"
	"syscall"
	"time"
	"usercenter/config"
	"usercenter/database/service"
	"usercenter/server"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

func startRecord() {
	if !config.RECORDENABLED { // 未启用则直接返回
		return
	}

	zones, err := service.GetZoneListInDB()
	if err != nil {
		log.Printf("Failed to get zone list in database: %s", err.Error())
		return
	}

	var wg sync.WaitGroup
	ticker := time.NewTicker(time.Duration(60*1000/config.ACCELERATIONRATIO) * time.Millisecond)
	defer ticker.Stop()

	preTime := time.Now().Round(time.Minute)

	for range ticker.C {
		curTime := preTime.Add(time.Minute)
		for zoneID, sites := range zones {
			for _, siteID := range sites {
				wg.Add(1)
				go func(zoneID string, siteID string, curTime time.Time) {
					defer wg.Done()
					// 1. 查询site正在使用中的实例数
					instances, err := service.RecordCountForSite(zoneID, siteID)
					if err != nil {
						log.Printf("Failed to get instance count for site %s: %v", siteID, err)
						return
					}
					// 2. 查询site过去一分钟登录失败的次数
					loginFailures, err := service.QueryLoginFailures(zoneID, siteID, curTime, time.Minute)
					if err != nil {
						log.Printf("Failed to get login failures for site %s: %v", siteID, err)
						return
					}
					fmt.Printf("%s: Site %s has %d instances now, and %d devices failed to log in last one minute\n", curTime.Format("2006-01-02 15:04:00"), siteID, instances, loginFailures)
					// 3. 插入最新数据
					err = service.InsertRecord(zoneID, siteID, curTime.Format("2006-01-02 15:04:00"), instances, loginFailures)
					if err != nil {
						log.Printf("Failed to insert record for site %s: %v", siteID, err)
					}
				}(zoneID, siteID, curTime)
			}
		}
		wg.Wait()
		preTime = curTime
	}
}

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

	go func() {
		err := errors.New("error")
		for err != nil {
			err = server.Serve(ctx, "0.0.0.0", config.USERCENTERPORT)
		}
		fmt.Printf("server started\n")
	}()

	// 创建分布式锁。
	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		config.K8SNAMSPACE,
		"usercenter-lock",
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
			OnStartedLeading: func(ctx context.Context) {
				startRecord()
			},
			OnStoppedLeading: func() {
				klog.Fatalf("leaderelection lost")
			},
		},
		WatchDog: nil,
		Name:     "usercenter",
	})

}
