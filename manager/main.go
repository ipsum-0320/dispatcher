package main

import (
	"context"
	"errors"
	"fmt"
	"manager/config"
	"manager/server"
	"os/signal"
	"syscall"
	"time"

	k8s_client "manager/k8s-client"

	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	run := func(ctx context.Context) {
		// OnStartedLeading 会传入 ctx，这里的 ctx 是传给 RunOrDie 的 ctx。
		// http server 应当监听 0.0.0.0。
		err := errors.New("error")
		for err != nil {
			err = server.Serve(ctx, "0.0.0.0", config.MANAGERPORT)
		}
		fmt.Printf("server started\n")
	}

	// 创建分布式锁。
	rl, err := resourcelock.New(
		resourcelock.LeasesResourceLock,
		config.K8SNAMSPACE,
		"manager-lock",
		k8s_client.LocalClient.CoreV1(),
		k8s_client.LocalClient.CoordinationV1(),
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
		Name:     "manager",
	})
}
