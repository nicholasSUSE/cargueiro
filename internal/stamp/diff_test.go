package stamp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUnifiedDiff(t *testing.T) {
	tests := []struct {
		name   string
		before string
		after  string
		want   string
	}{
		{
			name:   "nothing changes",
			before: "a\nb\n",
			after:  "a\nb\n",
			want:   "",
		},
		{
			name:   "one line replaced",
			before: "a\nb\nc\n",
			after:  "a\nB\nc\n",
			want:   "--- Chart.yaml\n+++ Chart.yaml\n@@ -1,3 +1,3 @@\n a\n-b\n+B\n c\n",
		},
		{
			name:   "lines put in at the end",
			before: "a\nb\n",
			after:  "a\nb\nc\nd\n",
			want:   "--- Chart.yaml\n+++ Chart.yaml\n@@ -1,2 +1,4 @@\n a\n b\n+c\n+d\n",
		},
		{
			name:   "only two lines of context on each side",
			before: "1\n2\n3\n4\n5\n6\n7\n8\n9\n",
			after:  "1\n2\n3\n4\nFIVE\n6\n7\n8\n9\n",
			want:   "--- Chart.yaml\n+++ Chart.yaml\n@@ -3,5 +3,5 @@\n 3\n 4\n-5\n+FIVE\n 6\n 7\n",
		},
		{
			name:   "two changes far apart are two hunks",
			before: "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n",
			after:  "ONE\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\nTWELVE\n",
			want: "--- Chart.yaml\n+++ Chart.yaml\n" +
				"@@ -1,3 +1,3 @@\n-1\n+ONE\n 2\n 3\n" +
				"@@ -10,3 +10,3 @@\n 10\n 11\n-12\n+TWELVE\n",
		},
		{
			name:   "two changes close together are one hunk",
			before: "1\n2\n3\n4\n5\n",
			after:  "ONE\n2\n3\n4\nFIVE\n",
			want:   "--- Chart.yaml\n+++ Chart.yaml\n@@ -1,5 +1,5 @@\n-1\n+ONE\n 2\n 3\n 4\n-5\n+FIVE\n",
		},
		{
			name:   "a file with no newline at the end",
			before: "a\nb",
			after:  "a\nc",
			want:   "--- Chart.yaml\n+++ Chart.yaml\n@@ -1,2 +1,2 @@\n a\n-b\n+c\n",
		},
		{
			name:   "windows line endings do not show",
			before: "a\r\nb\r\n",
			after:  "a\r\nB\r\n",
			want:   "--- Chart.yaml\n+++ Chart.yaml\n@@ -1,2 +1,2 @@\n a\n-b\n+B\n",
		},
		{
			name:   "an empty file gets lines",
			before: "",
			after:  "a\n",
			want:   "--- Chart.yaml\n+++ Chart.yaml\n@@ -1,0 +1,1 @@\n+a\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, unifiedDiff("Chart.yaml", tt.before, tt.after))
		})
	}
}

func TestPlanDiff(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "rancher-webhook-upstream", "rancher-webhook")
	edit(t, dir, "values.yaml", "tag: latest", "tag: v0.11.4")

	plan, err := Build(context.Background(), cfg, dir, Options{Version: "111.0.4+up0.11.4"})
	require.NoError(t, err)

	require.Equal(t, "--- Chart.yaml\n+++ Chart.yaml\n"+
		"@@ -2,5 +2,5 @@\n"+
		" name: rancher-webhook\n"+
		" description: ValidatingAdmissionWebhook for Rancher types\n"+
		"-version: \"999\"\n"+
		"+version: 111.0.4+up0.11.4\n"+
		" appVersion: \"999\"\n"+
		" annotations:\n"+
		"@@ -14,2 +14,5 @@\n"+
		"   catalog.cattle.io/kube-version: \"< 1.37.0-0\"\n"+
		"   catalog.cattle.io/managed: \"true\"\n"+
		"+  helm.sh/images: |\n"+
		"+    - name: rancher-webhook\n"+
		"+      image: rancher/rancher-webhook:v0.11.4\n", plan.Diff())
}

func TestPlanDiffWhenNothingChanges(t *testing.T) {
	cfg := loadRules(t)
	dir := copyChart(t, "fleet", "fleet-crd")

	plan, err := Build(context.Background(), cfg, dir, Options{Version: "110.0.3+up0.16.3"})

	require.NoError(t, err)
	require.Empty(t, plan.Diff())
}
