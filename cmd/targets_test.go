package cmd

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	myecs "github.com/jedipunkz/miniecs/internal/pkg/aws/ecs"
	"github.com/stretchr/testify/assert"
)

// fakeTargetLister records the calls fetchTargets makes so tests can
// assert which clusters and services are fetched. fetchTargets calls it
// from several goroutines, so every field access is guarded by mu.
type fakeTargetLister struct {
	clusters   []string
	services   map[string][]string
	targets    map[string][]myecs.Target
	listErr    error
	serviceErr error
	targetErr  error

	// serviceErrByCluster and targetErrByService ("cluster/service" keys)
	// override the shared errors per cluster and per service, so tests
	// can fail different fetches with different errors.
	serviceErrByCluster map[string]error
	targetErrByService  map[string]error

	// targetDelay, when set, runs inside the in-flight window of every
	// ListTargets call outside mu, so tests can vary the goroutine
	// interleaving.
	targetDelay func(cluster, service string)

	// targetGate, when > 0, makes every ListTargets call block on gate
	// until that many calls are in flight; the call that reaches the
	// threshold closes the gate and all of them proceed together. gate
	// is created by the test, so reading it needs no lock.
	targetGate int
	gate       chan struct{}
	gateOnce   sync.Once

	mu sync.Mutex

	gotListClusters int
	gotServices     []string
	gotTargetCalls  [][2]string

	inFlight    int
	maxInFlight int
}

func (f *fakeTargetLister) ListClusters(ctx context.Context) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotListClusters++
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.clusters, nil
}

func (f *fakeTargetLister) ListServices(ctx context.Context, cluster string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gotServices = append(f.gotServices, cluster)
	if err, ok := f.serviceErrByCluster[cluster]; ok {
		return nil, err
	}
	if f.serviceErr != nil {
		return nil, f.serviceErr
	}
	return f.services[cluster], nil
}

func (f *fakeTargetLister) ListTargets(ctx context.Context, cluster, service string) ([]myecs.Target, error) {
	// The call counts as in-flight for its whole body, including the
	// delay and the gate wait, so concurrent calls that are sleeping or
	// waiting together are all counted at the same time. The delay and
	// the gate wait run outside mu so blocked goroutines do not
	// serialize through the lock.
	f.mu.Lock()
	f.gotTargetCalls = append(f.gotTargetCalls, [2]string{cluster, service})
	f.inFlight++
	if f.inFlight > f.maxInFlight {
		f.maxInFlight = f.inFlight
	}
	openGate := f.targetGate > 0 && f.inFlight >= f.targetGate
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		f.inFlight--
		f.mu.Unlock()
	}()

	if openGate {
		f.gateOnce.Do(func() { close(f.gate) })
	}
	if f.gate != nil {
		<-f.gate
	}

	if f.targetDelay != nil {
		f.targetDelay(cluster, service)
	}

	if err, ok := f.targetErrByService[cluster+"/"+service]; ok {
		return nil, err
	}
	if f.targetErr != nil {
		return nil, f.targetErr
	}
	return f.targets[cluster+"/"+service], nil
}

// maxInFlight returns the highest number of ListTargets calls observed
// running at the same time.
func (f *fakeTargetLister) maxConcurrentTargets() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxInFlight
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
		wantTargets  []myecs.Target
		listErr      string
		serviceErr   string
		targetErr    string
		wantErr      string
	}{
		{
			name:      "empty flag fetches every cluster",
			clusters:  []string{"c1", "c2"},
			services:  map[string][]string{"c1": {"s1"}, "c2": {"s2"}},
			targets:   fullTargets,
			wantCalls: [][2]string{{"c1", "s1"}, {"c2", "s2"}},
			wantTargets: []myecs.Target{
				targetFor("c1", "s1"),
				targetFor("c2", "s2"),
			},
			wantServices: []string{"c1", "c2"},
		},
		{
			name:        "flag fetches only the named cluster",
			clusterFlag: "c2",
			clusters:    []string{"c1", "c2"},
			services:    map[string][]string{"c1": {"s1"}, "c2": {"s2"}},
			targets:     fullTargets,
			wantCalls:   [][2]string{{"c2", "s2"}},
			wantTargets: []myecs.Target{
				targetFor("c2", "s2"),
			},
			wantServices: []string{"c2"},
		},
		{
			name:        "unknown cluster fails without fetching",
			clusterFlag: "missing",
			clusters:    []string{"c1"},
			wantErr:     `cluster "missing" not found`,
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
				assert.Nil(t, got)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantTargets, got)
			}
			// The fan-out runs concurrently, so the order in which the
			// clusters and services were called is not deterministic.
			assert.ElementsMatch(t, tt.wantServices, client.gotServices)
			assert.ElementsMatch(t, tt.wantCalls, client.gotTargetCalls)
		})
	}
}

// TestFetchTargetsErrorAggregation checks that errors from concurrent
// goroutines are all kept: the fetches run concurrently, each failure is
// recorded by its pair index and the joined error lists every cause in
// pair order.
func TestFetchTargetsErrorAggregation(t *testing.T) {
	errService1 := errors.New("service boom 1")
	errService2 := errors.New("service boom 2")
	errTarget1 := errors.New("target boom 1")
	errTarget2 := errors.New("target boom 2")

	t.Run("service list errors from all clusters are joined", func(t *testing.T) {
		client := &fakeTargetLister{
			clusters: []string{"c1", "c2"},
			services: map[string][]string{"c1": {"s1"}, "c2": {"s2"}},
			serviceErrByCluster: map[string]error{
				"c1": errService1,
				"c2": errService2,
			},
		}

		got, err := fetchTargets(context.Background(), client, "")

		assert.Nil(t, got)
		assert.EqualError(t, err, "service boom 1\nservice boom 2")
		assert.ErrorIs(t, err, errService1)
		assert.ErrorIs(t, err, errService2)
	})

	t.Run("target errors from all services are joined", func(t *testing.T) {
		client := &fakeTargetLister{
			clusters: []string{"c1", "c2"},
			services: map[string][]string{"c1": {"s1"}, "c2": {"s2"}},
			targets: map[string][]myecs.Target{
				"c1/s1": {{Cluster: "c1", Service: "s1", Container: "app"}},
			},
			targetErrByService: map[string]error{
				"c1/s1": errTarget1,
				"c2/s2": errTarget2,
			},
		}

		got, err := fetchTargets(context.Background(), client, "")

		assert.Nil(t, got)
		assert.EqualError(t, err, "target boom 1\ntarget boom 2")
		assert.ErrorIs(t, err, errTarget1)
		assert.ErrorIs(t, err, errTarget2)
		// Every service was attempted despite the failures.
		assert.ElementsMatch(t, [][2]string{{"c1", "s1"}, {"c2", "s2"}}, client.gotTargetCalls)
	})
}

// TestFetchTargetsTargetsOrderIsStable checks that the concurrent fan-out
// keeps the output order deterministic: the targets come out in cluster
// order, then service order, then per-service target order, no matter how
// the goroutines complete. Later services get longer delays, so without
// index-based writes the completion order would differ from this order.
func TestFetchTargetsTargetsOrderIsStable(t *testing.T) {
	clusters := []string{"c1", "c2", "c3"}
	services := map[string][]string{
		"c1": {"s1", "s2"},
		"c2": {"s3", "s4"},
		"c3": {"s5", "s6"},
	}
	targets := map[string][]myecs.Target{}
	want := []myecs.Target{}
	for _, cluster := range clusters {
		for _, service := range services[cluster] {
			pairTargets := []myecs.Target{
				{Cluster: cluster, Service: service, Container: "app"},
				{Cluster: cluster, Service: service, Container: "sidecar"},
			}
			targets[cluster+"/"+service] = pairTargets
			want = append(want, pairTargets...)
		}
	}

	// Later services take longer, so the goroutines complete in reverse
	// start order while the results must stay in start order.
	for run := 0; run < 20; run++ {
		client := &fakeTargetLister{
			clusters: clusters,
			services: services,
			targets:  targets,
			targetDelay: func(cluster, service string) {
				serviceNumber, err := strconv.Atoi(strings.TrimPrefix(service, "s"))
				assert.NoError(t, err)
				time.Sleep(time.Duration(7-serviceNumber) * time.Millisecond)
			},
		}

		got, err := fetchTargets(context.Background(), client, "")

		assert.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

// TestFetchTargetsBoundedConcurrency checks that the fan-out really runs
// ListTargets calls in parallel and that the parallelism stays within
// maxConcurrentFetches. Every ListTargets call blocks at a barrier that
// opens once maxConcurrentFetches calls are in flight, so no wall-clock
// timing is involved: the barrier can only open when the fan-out reaches
// the limit, and the observed peak can only exceed the limit if the
// errgroup limit is not enforced.
func TestFetchTargetsBoundedConcurrency(t *testing.T) {
	const serviceCount = 2 * maxConcurrentFetches
	clusters := []string{"c1"}
	services := map[string][]string{"c1": {}}
	targets := map[string][]myecs.Target{}
	for i := 1; i <= serviceCount; i++ {
		service := "s" + strconv.Itoa(i)
		services["c1"] = append(services["c1"], service)
		targets["c1/"+service] = []myecs.Target{
			{Cluster: "c1", Service: service, Container: "app"},
		}
	}

	client := &fakeTargetLister{
		clusters:   clusters,
		services:   services,
		targets:    targets,
		targetGate: maxConcurrentFetches,
		gate:       make(chan struct{}),
	}

	got, err := fetchTargets(context.Background(), client, "")

	assert.NoError(t, err)
	assert.Len(t, got, serviceCount)
	assert.Equal(t, maxConcurrentFetches, client.maxConcurrentTargets())
}

// errOrNil turns an expected error message into an error, or nil for "".
func errOrNil(message string) error {
	if message == "" {
		return nil
	}
	return errors.New(message)
}
