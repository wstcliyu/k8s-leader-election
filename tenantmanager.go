package main

import (
	"context"
	"sync"
	"time"

	"k8s.io/klog"
)

type TenantManager struct {
	lock    sync.RWMutex
	tenants map[string]time.Time
}

func NewTenantManager() *TenantManager {
	return &TenantManager{tenants: make(map[string]time.Time)}
}

func (tm *TenantManager) PrintTenants(_ context.Context) {
	tm.lock.RLock()
	defer tm.lock.RUnlock()
	klog.Infof("Printing %d tenants!", len(tm.tenants))
	for tenant, acquireTime := range tm.tenants {
		klog.Infof("tenant: %s, acquireTime: %s", tenant, acquireTime)
	}
}

func (tm *TenantManager) AddTenant(leaseLockName string) {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	klog.Infof("Adding tenant: %s", leaseLockName)
	tm.tenants[leaseLockName] = time.Now()
}

func (tm *TenantManager) RemoveTenant(leaseLockName string) {
	tm.lock.Lock()
	defer tm.lock.Unlock()
	klog.Infof("Removing tenant: %s", leaseLockName)
	if _, ok := tm.tenants[leaseLockName]; ok {
		delete(tm.tenants, leaseLockName)
	} else {
		klog.Infof("Tenant does not exist: %s", leaseLockName)
	}
}
