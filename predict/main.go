package main

import (
	"context"
	"database/sql"
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
	"predict/mysql"
	_ "predict/mysql"
	"syscall"
	"time"
)

var (
	ns   = "default"
	host = "0.0.0.0"
	port = 8080
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

	// TODO: 需要查询数据库，获取所有的 zoneId。
	rows, err := mysql.DB.Query("")
	if err != nil {
		fmt.Printf("query failed, err:%v\n", err)
		return
	}
	defer func(query *sql.Rows) {
		err := query.Close()
		if err != nil {
			fmt.Printf("close query failed, err:%v\n", err)
		}
	}(rows)

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
			// TODO: 具体的 zoneId 要从数据库中去拿，这里的 Process 要多协程。
			err := Process("huadong")
			if err != nil {
				fmt.Printf("process failed, err:%v\n", err)
				return
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
