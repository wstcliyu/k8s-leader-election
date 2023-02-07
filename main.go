package main

import (
	"context"
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/wstcliyu/k8s-leader-election/priorityleaderelection"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog"
)

const (
	LeaseLockNamePrefix = "leaselock-"
	LeaseLockNamespace  = "default"
	RunTimeout          = time.Minute
	Probation           = time.Minute
)

func getId(lockname, podname string) string {
	h := sha256.New()
	h.Write([]byte(lockname))
	h.Write([]byte(podname))
	bs := h.Sum(nil)
	parts := []string{
		fmt.Sprintf("%x", bs),
		podname,
		lockname,
	}
	return strings.Join(parts, "/")
}

func getNewLock(client *clientset.Clientset, lockname, namespace, podname string) *resourcelock.LeaseLock {
	return &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      lockname,
			Namespace: namespace,
		},
		Client: client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: getId(lockname, podname),
		},
	}
}

func runLeaderElection(ctx context.Context,
	lock *resourcelock.LeaseLock,
	tm *TenantManager,
	startTimestamp time.Time,
	probation time.Duration,
	runTimeout time.Duration) {
	leaderElectionConfig := priorityleaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   15 * time.Second,
		RenewDeadline:   10 * time.Second,
		RetryPeriod:     2 * time.Second,
		StartTimestamp:  startTimestamp,
		Probation:       probation,
		RunTimeout:      runTimeout,
		Callbacks: priorityleaderelection.LeaderCallbacks{
			OnStartedLeading: func(c context.Context) {
				klog.Infof("For %s, start leading!", lock.LeaseMeta.Name)
				tm.AddTenant(lock.LeaseMeta.Name)
			},
			OnStoppedLeading: func() {
				klog.Infof("For %s, no longer the leader, staying inactive.", lock.LeaseMeta.Name)
				tm.RemoveTenant(lock.LeaseMeta.Name)
			},
			OnNewLeader: func(current_id string) {
				if current_id == lock.LockConfig.Identity {
					klog.Infof("For %s, still the leader!", lock.LeaseMeta.Name)
					return
				}
				klog.Infof("For %s, new leader is %s", lock.LeaseMeta.Name, current_id)
			},
		},
	}
	priorityleaderelection.RunPriorityLeaderElection(ctx, leaderElectionConfig)
}

func main() {
	startTimestamp := time.Now()
	var (
		leaseLockNum string
	)
	flag.StringVar(&leaseLockNum, "lease-num", "", "Number of lease lock")
	flag.Parse()

	tenantNum, err := strconv.Atoi(leaseLockNum)
	if err != nil {
		klog.Fatalf("failed to convert leaseLockNum to int")
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		klog.Fatalf("failed to get kubeconfig")
	}
	config.QPS = 500.0
	config.Burst = 1000.0

	client, err := clientset.NewForConfig(config)
	if err != nil {
		klog.Fatalf("failed to get kubeclient")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20 * time.Minute)
	defer cancel()

	tm := NewTenantManager()
	podName := os.Getenv("POD_NAME")
	for i := 0; i < tenantNum; i++ {
		leaseLockName := LeaseLockNamePrefix + strconv.Itoa(i)
		lock := getNewLock(client, leaseLockName, LeaseLockNamespace, podName)
		go runLeaderElection(ctx, lock, tm, startTimestamp, Probation, RunTimeout)
	}
	wait.UntilWithContext(ctx, tm.PrintTenants, 5 * time.Second)
}
