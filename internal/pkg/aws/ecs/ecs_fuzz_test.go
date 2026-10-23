package ecs

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
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

	f.Fuzz(func(t *testing.T, arn string) {
		names, err := parseClusterARNs([]string{arn})
		if arn == "" {
			if err == nil {
				t.Fatalf("expected error for empty ARN")
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", arn, err)
		}
		if len(names) != 1 {
			t.Fatalf("got %d names, want 1", len(names))
		}
		name := names[0]
		if strings.Contains(name, "/") {
			t.Fatalf("name %q contains '/'", name)
		}
		if !strings.HasSuffix(arn, name) {
			t.Fatalf("name %q is not a suffix of %q", name, arn)
		}
	})
}

func FuzzParseServiceARNs(f *testing.F) {
	f.Add("arn:aws:ecs:ap-northeast-1:123456789012:service/my-cluster/my-service")
	f.Add("arn:aws:ecs:ap-northeast-1:123456789012:service/my-service")
	f.Add("no-slash")
	f.Add("trailing/")
	f.Add("")

	f.Fuzz(func(t *testing.T, arn string) {
		names, err := parseServiceARNs([]string{arn})
		if arn == "" {
			if err == nil {
				t.Fatalf("expected error for empty ARN")
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", arn, err)
		}
		if len(names) != 1 {
			t.Fatalf("got %d names, want 1", len(names))
		}
		name := names[0]
		if strings.Contains(name, "/") {
			t.Fatalf("name %q contains '/'", name)
		}
		if !strings.HasSuffix(arn, name) {
			t.Fatalf("name %q is not a suffix of %q", name, arn)
		}
	})
}

// parseTaskContainers feeds the CLI its container names, images and
// statuses straight from DescribeTasks. Whatever the API returns, a
// container without a name must be rejected and a valid one must keep
// its fields verbatim (empty string for missing optionals).
func FuzzParseTaskContainers(f *testing.F) {
	f.Add("app", "nginx:1", "RUNNING")
	f.Add("app", "", "")
	f.Add("", "nginx:1", "")

	f.Fuzz(func(t *testing.T, name, image, status string) {
		// An empty fuzz name means "no name": the parse must reject it
		// the same way it rejects a nil name.
		container := types.Container{Image: aws.String(image), LastStatus: aws.String(status)}
		if name != "" {
			container.Name = aws.String(name)
		}
		parsed, err := parseTaskContainers([]types.Container{container})
		if name == "" {
			if err == nil {
				t.Fatal("expected error for container without name")
			}
			return
		}
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := containerInfo{Name: name, Image: image, LastStatus: status}
		if parsed[0] != want {
			t.Fatalf("parsed = %+v, want %+v", parsed[0], want)
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

	c := &Client{}
	f.Fuzz(func(t *testing.T, target string) {
		b, err := c.buildSSMTargetJSON(target)
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
