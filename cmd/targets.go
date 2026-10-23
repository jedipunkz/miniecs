package cmd

import (
	"context"
	"fmt"

	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
)

// ecsTargetLister is the subset of *myecs.Client that the login and list
// flows use. Defining it here keeps both commands testable with a
// cmd-local fake instead of a real AWS client.
type ecsTargetLister interface {
	ListClusters(ctx context.Context) ([]string, error)
	ListServices(ctx context.Context, cluster string) ([]string, error)
	ListTargets(ctx context.Context, cluster, service string) ([]myecs.Target, error)
}

// selectTargetClusters returns the clusters to fetch targets for. With an
// empty name it returns all clusters; otherwise it returns only the named
// cluster and fails when that cluster does not exist.
func selectTargetClusters(allClusters []string, name string) ([]string, error) {
	if name == "" {
		return allClusters, nil
	}

	for _, cluster := range allClusters {
		if cluster == name {
			return []string{cluster}, nil
		}
	}

	return nil, fmt.Errorf("cluster %q not found", name)
}

// fetchTargets lists every container of every service of the clusters
// selected by the cluster name and returns them as flat targets.
func fetchTargets(ctx context.Context, ecsClient ecsTargetLister, clusterName string) ([]myecs.Target, error) {
	allClusters, err := ecsClient.ListClusters(ctx)
	if err != nil {
		return nil, err
	}

	targetClusters, err := selectTargetClusters(allClusters, clusterName)
	if err != nil {
		return nil, err
	}

	var targets []myecs.Target
	for _, cluster := range targetClusters {
		services, err := ecsClient.ListServices(ctx, cluster)
		if err != nil {
			return nil, err
		}

		for _, service := range services {
			loaded, err := ecsClient.ListTargets(ctx, cluster, service)
			if err != nil {
				return nil, err
			}
			targets = append(targets, loaded...)
		}
	}

	return targets, nil
}
