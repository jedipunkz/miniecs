package cmd

import (
	"context"
	"errors"
	"fmt"

	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"golang.org/x/sync/errgroup"
)

// maxConcurrentFetches bounds how many ECS API calls fetchTargets runs in
// parallel. Of the 8-16 range the issue proposes, the conservative end
// keeps a large fan-out below the per-account ECS API throttling limits.
const maxConcurrentFetches = 8

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

// clusterService is one (cluster, service) pair whose targets are fetched.
type clusterService struct {
	cluster string
	service string
}

// fetchTargets lists every container of every service of the clusters
// selected by the cluster name and returns them as flat targets.
//
// The per-cluster service lists and the per-service targets are fetched
// concurrently with a bounded errgroup. Every goroutine writes only its
// own index in pre-sized slices, so the result order stays deterministic:
// cluster order as returned by ListClusters, then service order as
// returned by ListServices, then the order ListTargets returns for that
// service — the same order the previous serial walk produced.
//
// Errors from the goroutines are collected by index and aggregated with
// errors.Join. All fetches are attempted even when some fail, and no
// partial targets are returned when any fails, matching the serial code
// that returned no targets once a call errored.
func fetchTargets(ctx context.Context, ecsClient ecsTargetLister, clusterName string) ([]myecs.Target, error) {
	allClusters, err := ecsClient.ListClusters(ctx)
	if err != nil {
		return nil, err
	}

	targetClusters, err := selectTargetClusters(allClusters, clusterName)
	if err != nil {
		return nil, err
	}

	pairs, err := listClusterServices(ctx, ecsClient, targetClusters)
	if err != nil {
		return nil, err
	}

	return listServiceTargets(ctx, ecsClient, pairs)
}

// listClusterServices lists the services of every cluster concurrently and
// returns the (cluster, service) pairs in cluster order. The goroutines
// record their results and errors at their own index in pre-sized slices
// instead of returning them, so all errors can be joined in a stable
// order instead of errgroup keeping only the first one.
func listClusterServices(ctx context.Context, ecsClient ecsTargetLister, clusters []string) ([]clusterService, error) {
	servicesByIndex := make([][]string, len(clusters))
	errs := make([]error, len(clusters))

	var g errgroup.Group
	g.SetLimit(maxConcurrentFetches)
	for i, cluster := range clusters {
		g.Go(func() error {
			services, err := ecsClient.ListServices(ctx, cluster)
			servicesByIndex[i] = services
			errs[i] = err
			return nil
		})
	}
	_ = g.Wait() // always nil: the goroutines record errors by index

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	var pairs []clusterService
	for i, cluster := range clusters {
		for _, service := range servicesByIndex[i] {
			pairs = append(pairs, clusterService{cluster: cluster, service: service})
		}
	}
	return pairs, nil
}

// listServiceTargets fetches the targets of every (cluster, service) pair
// concurrently and returns them concatenated in pair order. The goroutines
// record their results and errors at their own index in pre-sized slices
// instead of returning them, so all errors can be joined in a stable order
// instead of errgroup keeping only the first one.
func listServiceTargets(ctx context.Context, ecsClient ecsTargetLister, pairs []clusterService) ([]myecs.Target, error) {
	targetsByIndex := make([][]myecs.Target, len(pairs))
	errs := make([]error, len(pairs))

	var g errgroup.Group
	g.SetLimit(maxConcurrentFetches)
	for i, pair := range pairs {
		g.Go(func() error {
			loaded, err := ecsClient.ListTargets(ctx, pair.cluster, pair.service)
			targetsByIndex[i] = loaded
			errs[i] = err
			return nil
		})
	}
	_ = g.Wait() // always nil: the goroutines record errors by index

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	var targets []myecs.Target
	for _, loaded := range targetsByIndex {
		targets = append(targets, loaded...)
	}
	return targets, nil
}
