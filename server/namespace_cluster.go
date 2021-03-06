// Copyright 2017 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package server

import (
	"math/rand"

	"github.com/pingcap/pd/server/core"
	"github.com/pingcap/pd/server/namespace"
	"github.com/pingcap/pd/server/schedule"
	"github.com/pingcap/pd/server/schedule/operator"
	"github.com/pingcap/pd/server/schedule/opt"
	"github.com/pingcap/pd/server/statistics"
)

// namespaceCluster is part of a global cluster that contains stores and regions
// within a specific namespace.
type namespaceCluster struct {
	opt.Cluster
	classifier namespace.Classifier
	namespace  string
	stores     map[uint64]*core.StoreInfo
}

func newNamespaceCluster(c opt.Cluster, classifier namespace.Classifier, namespace string) *namespaceCluster {
	stores := make(map[uint64]*core.StoreInfo)
	for _, s := range c.GetStores() {
		if classifier.GetStoreNamespace(s) == namespace {
			stores[s.GetID()] = s
		}
	}
	return &namespaceCluster{
		Cluster:    c,
		classifier: classifier,
		namespace:  namespace,
		stores:     stores,
	}
}

func (c *namespaceCluster) checkRegion(region *core.RegionInfo) bool {
	if c.classifier.GetRegionNamespace(region) != c.namespace {
		return false
	}
	for _, p := range region.GetPeers() {
		if _, ok := c.stores[p.GetStoreId()]; !ok {
			return false
		}
	}
	return true
}

const randRegionMaxRetry = 10

// RandFollowerRegion returns a random region that has a follower on the store.
func (c *namespaceCluster) RandFollowerRegion(storeID uint64, opts ...core.RegionOption) *core.RegionInfo {
	for i := 0; i < randRegionMaxRetry; i++ {
		r := c.Cluster.RandFollowerRegion(storeID, opts...)
		if r == nil {
			return nil
		}
		if c.checkRegion(r) {
			return r
		}
	}
	return nil
}

// RandLeaderRegion returns a random region that has leader on the store.
func (c *namespaceCluster) RandLeaderRegion(storeID uint64, opts ...core.RegionOption) *core.RegionInfo {
	for i := 0; i < randRegionMaxRetry; i++ {
		r := c.Cluster.RandLeaderRegion(storeID, opts...)
		if r == nil {
			return nil
		}
		if c.checkRegion(r) {
			return r
		}
	}
	return nil
}

// GetAverageRegionSize returns the average region approximate size.
func (c *namespaceCluster) GetAverageRegionSize() int64 {
	var totalCount, totalSize int64
	for _, s := range c.stores {
		totalCount += int64(s.GetRegionCount())
		totalSize += s.GetRegionSize()
	}
	if totalCount == 0 {
		return 0
	}
	return totalSize / totalCount
}

// GetStores returns all stores in the namespace.
func (c *namespaceCluster) GetStores() []*core.StoreInfo {
	stores := make([]*core.StoreInfo, 0, len(c.stores))
	for _, s := range c.stores {
		stores = append(stores, s)
	}
	return stores
}

// GetStore searches for a store by ID.
func (c *namespaceCluster) GetStore(id uint64) *core.StoreInfo {
	return c.stores[id]
}

// GetRegion searches for a region by ID.
func (c *namespaceCluster) GetRegion(id uint64) *core.RegionInfo {
	r := c.Cluster.GetRegion(id)
	if r == nil || !c.checkRegion(r) {
		return nil
	}
	return r
}

// RegionWriteStats returns hot region's write stats.
func (c *namespaceCluster) RegionWriteStats() map[uint64][]*statistics.HotPeerStat {
	return c.Cluster.RegionWriteStats()
}

func scheduleByNamespace(cluster opt.Cluster, classifier namespace.Classifier, scheduler schedule.Scheduler) []*operator.Operator {
	namespaces := classifier.GetAllNamespaces()
	for _, i := range rand.Perm(len(namespaces)) {
		nc := newNamespaceCluster(cluster, classifier, namespaces[i])
		if op := scheduler.Schedule(nc); op != nil {
			return op
		}
	}
	return nil
}

func (c *namespaceCluster) GetLeaderScheduleLimit() uint64 {
	return c.GetOpt().GetLeaderScheduleLimit(c.namespace)
}

func (c *namespaceCluster) GetRegionScheduleLimit() uint64 {
	return c.GetOpt().GetRegionScheduleLimit(c.namespace)
}

func (c *namespaceCluster) GetReplicaScheduleLimit() uint64 {
	return c.GetOpt().GetReplicaScheduleLimit(c.namespace)
}

func (c *namespaceCluster) GetMergeScheduleLimit() uint64 {
	return c.GetOpt().GetMergeScheduleLimit(c.namespace)
}

func (c *namespaceCluster) GetMaxReplicas() int {
	return c.GetOpt().GetMaxReplicas(c.namespace)
}
