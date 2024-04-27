package main

import (
	"context"
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"manager/k8s-client"
	"manager/server"
	"os/signal"
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

	c := k8s_client.Client

	id := string(uuid.NewUUID())

	run := func(ctx context.Context) {
		// OnStartedLeading 会传入 ctx，这里的 ctx 是传给 RunOrDie 的 ctx。
		// http server 应当监听 0.0.0.0。
		err := errors.New("error")
		for err != nil {
			err = server.Serve(ctx, host, int64(port))
		}
		fmt.Printf("server started\n")
	}

	// 创建分布式锁。
	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		ns,
		"manager-lock",
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
		Name:     "manager",
	})
}
