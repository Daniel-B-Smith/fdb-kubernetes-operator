/*
 * update_config_map.go
 *
 * This source file is part of the FoundationDB open source project
 *
 * Copyright 2019 Apple Inc. and the FoundationDB project authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package controllers

import (
	ctx "context"
	"time"

	fdbtypes "github.com/FoundationDB/fdb-kubernetes-operator/api/v1beta1"
)

// UpdatePodConfig provides a reconciliation step for updating the dynamic conf
// for a all pods.
type UpdatePodConfig struct{}

// Reconcile runs the reconciler's work.
func (u UpdatePodConfig) Reconcile(r *FoundationDBClusterReconciler, context ctx.Context, cluster *fdbtypes.FoundationDBCluster) (bool, error) {
	configMap, err := GetConfigMap(cluster)
	if err != nil {
		return false, err
	}

	configMapHash, err := GetDynamicConfHash(configMap)
	if err != nil {
		return false, err
	}

	instances, err := r.PodLifecycleManager.GetInstances(r, cluster, context, getPodListOptions(cluster, "", "")...)
	if err != nil {
		return false, err
	}

	allSynced := true
	// We try to update all instances and if we observe an error we add it to the error list.
	for index := range instances {
		instance := instances[index]
		if instance.Metadata.Annotations[fdbtypes.LastConfigMapKey] == configMapHash {
			continue
		}

		synced, err := r.updatePodDynamicConf(cluster, instance)
		if !synced {
			allSynced = false
			log.Info("Update dynamic Pod config", "namespace", cluster.Namespace, "cluster", cluster.Name, "processGroupID", instance.GetInstanceID(), "synced", synced, "error", err)
			continue
		}

		instance.Metadata.Annotations[fdbtypes.LastConfigMapKey] = configMapHash
		err = r.PodLifecycleManager.UpdateMetadata(r, context, cluster, instance)
		if err != nil {
			allSynced = false
			log.Info("Update Pod metadata", "namespace", configMap.Namespace, "cluster", cluster.Name, "processGroupID", instance.GetInstanceID(), "error", err)
		}
	}

	// If we return an error we don't requeue
	// So we just return that we can't continue but don't have an error
	return allSynced, nil
}

// RequeueAfter returns the delay before we should run the reconciliation
// again.
func (u UpdatePodConfig) RequeueAfter() time.Duration {
	return 15 * time.Second
}
