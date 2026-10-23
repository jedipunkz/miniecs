package cmd

import (
	"context"
	"errors"
	"testing"

	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/stretchr/testify/assert"
)

// fakeTargetLister records the calls fetchTargets makes so tests can
// assert which clusters and services are fetched.
type fakeTargetLister struct {
	clusters   []string
	services   map[string][]string
	targets    map[string][]myecs.Target
	listErr    error
	serviceErr error
	targetErr  error

	gotListClusters int
	gotServices     []string
	gotTargetCalls  [][2]string
}

func (f *fakeTargetLister) ListClusters(ctx context.Context) ([]string, error) {
	f.gotListClusters++
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.clusters, nil
}

func (f *fakeTargetLister) ListServices(ctx context.Context, cluster string) ([]string, error) {
	f.gotServices = append(f.gotServices, cluster)
	if f.serviceErr != nil {
		return nil, f.serviceErr
	}
	return f.services[cluster], nil
}

func (f *fakeTargetLister) ListTargets(ctx context.Context, cluster, service string) ([]myecs.Target, error) {
	f.gotTargetCalls = append(f.gotTargetCalls, [2]string{cluster, service})
	if f.targetErr != nil {
		return nil, f.targetErr
	}
	return f.targets[cluster+"/"+service], nil
}

func TestSelectTargetClusters(t *testing.T) {
	tests := []struct {
		name         string
		clusterFlag  string
		allClusters  []string
		wantClusters []string
		wantErr      string
	}{
		{
			name:         "empty flag selects all clusters",
			clusterFlag:  "",
			allClusters:  []string{"c1", "c2"},
			wantClusters: []string{"c1", "c2"},
		},
		{
			name:         "flag selects the named cluster only",
			clusterFlag:  "c2",
			allClusters:  []string{"c1", "c2", "c3"},
			wantClusters: []string{"c2"},
		},
		{
			name:         "unknown cluster returns an error",
			clusterFlag:  "missing",
			allClusters:  []string{"c1"},
			wantClusters: nil,
			wantErr:      `cluster "missing" not found`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := selectTargetClusters(tt.allClusters, tt.clusterFlag)

			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
				assert.Nil(t, got)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.wantClusters, got)
		})
	}
}

func TestFetchTargets(t *testing.T) {
	targetFor := func(cluster, service string) myecs.Target {
		return myecs.Target{Cluster: cluster, Service: service, Container: "container-1"}
	}
	fullTargets := map[string][]myecs.Target{
		"c1/s1": {targetFor("c1", "s1")},
		"c2/s2": {targetFor("c2", "s2")},
	}

	tests := []struct {
		name         string
		clusterFlag  string
		clusters     []string
		services     map[string][]string
		targets      map[string][]myecs.Target
		wantServices []string
		wantCalls    [][2]string
		wantCount    int
		listErr      string
		serviceErr   string
		targetErr    string
		wantErr      string
	}{
		{
			name:         "empty flag fetches every cluster",
			clusters:     []string{"c1", "c2"},
			services:     map[string][]string{"c1": {"s1"}, "c2": {"s2"}},
			targets:      fullTargets,
			wantServices: []string{"c1", "c2"},
			wantCalls:    [][2]string{{"c1", "s1"}, {"c2", "s2"}},
			wantCount:    2,
		},
		{
			name:         "flag fetches only the named cluster",
			clusterFlag:  "c2",
			clusters:     []string{"c1", "c2"},
			services:     map[string][]string{"c1": {"s1"}, "c2": {"s2"}},
			targets:      fullTargets,
			wantServices: []string{"c2"},
			wantCalls:    [][2]string{{"c2", "s2"}},
			wantCount:    1,
		},
		{
			name:         "unknown cluster fails without fetching",
			clusterFlag:  "missing",
			clusters:     []string{"c1"},
			wantServices: nil,
			wantCalls:    nil,
			wantCount:    0,
			wantErr:      `cluster "missing" not found`,
		},
		{
			name:     "cluster list error is returned",
			clusters: []string{"c1"},
			listErr:  "boom",
			wantErr:  "boom",
		},
		{
			name:         "service list error is returned",
			clusters:     []string{"c1"},
			services:     map[string][]string{"c1": {"s1"}},
			serviceErr:   "boom",
			wantServices: []string{"c1"},
			wantErr:      "boom",
		},
		{
			name:         "target list error is returned",
			clusters:     []string{"c1"},
			services:     map[string][]string{"c1": {"s1"}},
			targetErr:    "boom",
			wantServices: []string{"c1"},
			wantCalls:    [][2]string{{"c1", "s1"}},
			wantErr:      "boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeTargetLister{
				clusters:   tt.clusters,
				services:   tt.services,
				targets:    tt.targets,
				listErr:    errOrNil(tt.listErr),
				serviceErr: errOrNil(tt.serviceErr),
				targetErr:  errOrNil(tt.targetErr),
			}

			got, err := fetchTargets(context.Background(), client, tt.clusterFlag)

			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantServices, client.gotServices)
			assert.Equal(t, tt.wantCalls, client.gotTargetCalls)
			assert.Equal(t, tt.wantCount, len(got))
		})
	}
}

// errOrNil turns an expected error message into an error, or nil for "".
func errOrNil(message string) error {
	if message == "" {
		return nil
	}
	return errors.New(message)
}
