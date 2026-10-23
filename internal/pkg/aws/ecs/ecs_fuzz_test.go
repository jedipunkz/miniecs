package ecs

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

// The fuzz targets check invariants, not only the absence of panics.
// CI runs the seed corpus as regular tests; run longer locally with e.g.
//   go test -run '^$' -fuzz FuzzParseClusterARNs -fuzztime 30s ./internal/pkg/aws/ecs

func FuzzParseClusterARNs(f *testing.F) {
	f.Add("arn:aws:ecs:ap-northeast-1:123456789012:cluster/my-cluster")
	f.Add("arn:aws:ecs:us-east-1:123456789012:cluster/a/b")
	f.Add("no-slash")
	f.Add("trailing/")
	f.Add("")

	e := &ECSResource{}
	f.Fuzz(func(t *testing.T, arn string) {
		clusters, err := e.parseClusterARNs([]string{arn})
		if arn == "" {
			if err == nil {
				t.Fatalf("expected error for empty ARN")
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", arn, err)
		}
		if len(clusters) != 1 {
			t.Fatalf("got %d clusters, want 1", len(clusters))
		}
		c := clusters[0]
		if c.ClusterArn != arn {
			t.Fatalf("ClusterArn = %q, want %q", c.ClusterArn, arn)
		}
		if strings.Contains(c.ClusterName, "/") {
			t.Fatalf("ClusterName %q contains '/'", c.ClusterName)
		}
		if !strings.HasSuffix(arn, c.ClusterName) {
			t.Fatalf("ClusterName %q is not a suffix of %q", c.ClusterName, arn)
		}
		if c.Services == nil {
			t.Fatalf("Services is nil")
		}
	})
}

func FuzzParseServiceARNs(f *testing.F) {
	f.Add("arn:aws:ecs:ap-northeast-1:123456789012:service/my-cluster/my-service", "my-cluster")
	f.Add("arn:aws:ecs:ap-northeast-1:123456789012:service/my-service", "")
	f.Add("", "c")

	e := &ECSResource{}
	f.Fuzz(func(t *testing.T, arn, cluster string) {
		services, err := e.parseServiceARNs([]string{arn}, cluster)
		if arn == "" {
			if err == nil {
				t.Fatalf("expected error for empty ARN")
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", arn, err)
		}
		if len(services) != 1 {
			t.Fatalf("got %d services, want 1", len(services))
		}
		s := services[0]
		if s.ServiceArn != arn || s.ClusterName != cluster {
			t.Fatalf("fields not preserved: %+v", s)
		}
		if strings.Contains(s.ServiceName, "/") {
			t.Fatalf("ServiceName %q contains '/'", s.ServiceName)
		}
		if !strings.HasSuffix(arn, s.ServiceName) {
			t.Fatalf("ServiceName %q is not a suffix of %q", s.ServiceName, arn)
		}
	})
}

// The target JSON is passed to session-manager-plugin. Whatever the
// cluster, task or container names contain, it must stay valid JSON
// with exactly one Target field equal to the input.
func FuzzBuildSSMTargetJSON(f *testing.F) {
	f.Add("ecs:cluster_task_container")
	f.Add(`ecs:"quoted"_\back_slash`)
	f.Add("ecs:\x00\n\t")

	e := &ECSResource{}
	f.Fuzz(func(t *testing.T, target string) {
		b, err := e.buildSSMTargetJSON(target)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var got map[string]string
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("output is not valid JSON: %v: %s", err, b)
		}
		if len(got) != 1 {
			t.Fatalf("got %d keys, want 1: %s", len(got), b)
		}
		// encoding/json replaces each invalid UTF-8 byte with U+FFFD, so
		// exact equality only holds for valid UTF-8 input.
		if utf8.ValidString(target) {
			if got["Target"] != target {
				t.Fatalf("Target = %q, want %q", got["Target"], target)
			}
		} else if !utf8.ValidString(got["Target"]) {
			t.Fatalf("Target %q is not valid UTF-8", got["Target"])
		}
	})
}
