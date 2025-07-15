package dashboardapi

import (
	"context"

	v1 "github.com/rancher/rancher/pkg/generated/controllers/provisioning.cattle.io/v1"
	"github.com/sirupsen/logrus"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/rancher/rancher/pkg/controllers/dashboard/helm"
	"github.com/rancher/rancher/pkg/controllers/dashboardapi/feature"
	"github.com/rancher/rancher/pkg/controllers/dashboardapi/settings"
	"github.com/rancher/rancher/pkg/wrangler"
)

func Register(ctx context.Context, wrangler *wrangler.Context) error {
	feature.Register(ctx, wrangler.Mgmt.Feature())
	helm.RegisterReposForFollowers(ctx, wrangler.Core.Secret().Cache(), wrangler.Catalog.ClusterRepo())

	s := machineDeploymentReplicaOverrider{
		clusterCache:      wrangler.Provisioning.Cluster().Cache(),
		clusterController: wrangler.Provisioning.Cluster(),
	}

	// hacky stuff
	wrangler.CAPI.MachineDeployment().OnChange(ctx, "testing", s.watch)

	return settings.Register(wrangler.Mgmt.Setting())
}

type machineDeploymentReplicaOverrider struct {
	clusterCache      v1.ClusterCache
	clusterController v1.ClusterController
}

func (s *machineDeploymentReplicaOverrider) watch(_ string, md *capi.MachineDeployment) (*capi.MachineDeployment, error) {
	if md == nil {
		return nil, nil
	}

	clusterName := md.Spec.Template.ObjectMeta.Labels["cluster.x-k8s.io/cluster-name"]
	if clusterName == "" {
		logrus.Debugf("MachineDeployment %s/%s has no cluster name label, skipping", md.Namespace, md.Name)
		return md, nil
	}

	machinePoolName := md.Spec.Template.ObjectMeta.Labels["rke.cattle.io/rke-machine-pool-name"]
	if machinePoolName == "" {
		logrus.Debugf("MachineDeployment %s/%s has no machine pool name label, skipping", md.Namespace, md.Name)
		return md, nil
	}

	logrus.Debugf("Getting cluster %s/%s", md.Namespace, clusterName)
	cluster, err := s.clusterCache.Get(md.Namespace, clusterName)
	if err != nil {
		logrus.Errorf("Error getting cluster %s/%s: %v", md.Namespace, clusterName, err)
		return md, err
	}

	needUpdate := false
	for i := range cluster.Spec.RKEConfig.MachinePools {
		if !(cluster.Spec.RKEConfig.MachinePools[i].Name == machinePoolName) {
			continue
		}

		logrus.Debugf("Found matching machine pool %s", machinePoolName)
		if *cluster.Spec.RKEConfig.MachinePools[i].Quantity != *md.Spec.Replicas {
			logrus.Infof("Updating cluster %s/%s machine pool %s quantity from %d to %d",
				cluster.Namespace, cluster.Name, machinePoolName,
				*cluster.Spec.RKEConfig.MachinePools[i].Quantity, *md.Spec.Replicas)
			cluster.Spec.RKEConfig.MachinePools[i].Quantity = md.Spec.Replicas
			needUpdate = true
		}
	}

	if needUpdate {
		logrus.Debugf("Updating cluster %s/%s", cluster.Namespace, cluster.Name)
		_, err = s.clusterController.Update(cluster)
		if err != nil {
			logrus.Warnf("Failed to update cluster %s/%s to match machineDeployment! %v",
				cluster.Namespace, cluster.Name, err)
			return md, err
		}
		logrus.Infof("Successfully updated cluster %s/%s", cluster.Namespace, cluster.Name)
	}

	return md, nil
}
