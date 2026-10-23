package cmd

import (
	"strings"
	"testing"
)

// FuzzShortenTaskDefArn checks the invariants of the task definition short
// form for arbitrary input: the result is never empty, is a suffix of the
// input, and when the input carries the task-definition ARN marker with a
// non-empty remainder, the remainder is rendered verbatim.
// CI runs the seed corpus as regular tests; run longer locally with e.g.
//
//	go test -run '^$' -fuzz FuzzShortenTaskDefArn -fuzztime 30s ./cmd
func FuzzShortenTaskDefArn(f *testing.F) {
	f.Add("arn:aws:ecs:ap-northeast-1:123456789012:task-definition/api:42")
	f.Add("arn:aws:ecs:us-east-1:123456789012:task-definition/teamA/web:1")
	f.Add("arn:aws:ecs:us-east-1:123456789012:task-definition/")
	f.Add("web:1")
	f.Add("")
	f.Add("arn:task-definition/:")

	f.Fuzz(func(t *testing.T, taskDefinition string) {
		got := shortenTaskDefArn(taskDefinition)

		if got == "" && taskDefinition != "" {
			t.Fatalf("empty result for non-empty input %q", taskDefinition)
		}
		index := strings.Index(taskDefinition, taskDefinitionARNMarker)
		if index >= 0 {
			remainder := taskDefinition[index+len(taskDefinitionARNMarker):]
			if remainder != "" {
				if got != remainder {
					t.Fatalf("marker with remainder %q rendered as %q", remainder, got)
				}
				return
			}
		}
		if got != taskDefinition {
			t.Fatalf("input %q without marker remainder changed to %q", taskDefinition, got)
		}
	})
}
