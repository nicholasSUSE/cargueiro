package annotations

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func parseValues(t *testing.T, text string) map[string]any {
	t.Helper()

	var values map[string]any
	require.NoError(t, yaml.Unmarshal([]byte(text), &values))
	return values
}

func TestCheckValuesYamlValid(t *testing.T) {
	tests := []struct {
		name   string
		values string
	}{
		// What the released charts carry.
		{
			name: "fleet",
			values: `
image:
  repository: rancher/fleet
  tag: v0.16.3
agentImage:
  repository: rancher/fleet-agent
  tag: v0.16.3
`,
		},
		{
			name: "longhorn, with an empty registry beside the image",
			values: `
image:
  longhorn:
    engine:
      registry: ""
      repository: rancher/mirrored-longhornio-longhorn-engine
      tag: v1.12.1
`,
		},
		{
			name: "longhorn, OpenShift image left empty",
			values: `
image:
  openshift:
    oauthProxy:
      registry: ""
      repository: ""
      tag: ""
`,
		},
		{
			name: "rancher-logging, repository and tag with no value at all",
			values: `
eventTailer:
  image:
    repository:
    tag:
    pullPolicy:
`,
		},
		{
			name: "neuvector, with optional keys beside the image",
			values: `
controller:
  image:
    repository: rancher/neuvector-controller
    tag: 5.6.4
    hash:
    imagePullPolicy: IfNotPresent
`,
		},
		{
			name: "rancher-monitoring, with a sha beside the image",
			values: `
alertmanager:
  alertmanagerSpec:
    image:
      repository: rancher/appco-alertmanager
      tag: 0.30.0-12.16
      sha: ""
`,
		},

		// Where an image may be.
		{
			name: "image at the top level",
			values: `
repository: rancher/a
tag: "1"
`,
		},
		{
			name: "image inside a list",
			values: `
containers:
  - name: a
    image:
      repository: rancher/a
      tag: "1"
`,
		},
		{
			name: "several images",
			values: `
a:
  repository: rancher/a
  tag: "1"
b:
  repository: rancher/b
  tag: "2"
`,
		},

		// Nothing that looks like an image.
		{
			name:   "no values",
			values: ``,
		},
		{
			name: "values without images",
			values: `
replicas: 2
service:
  port: 8080
  names: [a, b]
`,
		},
		{
			name: "a key called image that is a map",
			values: `
image:
  pullPolicy: Always
`,
		},
		{
			name: "a key called image that holds a string is not an image",
			values: `
testReceiver:
  image: fluent/fluent-bit
`,
		},
		{
			name: "a flat string with a tag is not an image either",
			values: `
image: rancher/a:1
`,
		},
		{
			name: "a flat string with a registry",
			values: `
defaultChannels:
  base:
    image: registry.suse.com/rancher/elemental-channel/sl-micro:6.2-base
`,
		},
		{
			name: "a blank image string",
			values: `
image: ""
`,
		},
		{
			name: "a tag without a repository key is not an image block",
			values: `
version:
  tag: v1
`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Empty(t, checkValuesYaml(parseValues(t, tt.values)))
		})
	}

	require.Empty(t, checkValuesYaml(nil))
	require.Empty(t, checkValuesYaml(map[string]any{}))
}

func TestCheckValuesYamlInvalid(t *testing.T) {
	tests := []struct {
		name   string
		values string
		want   []string
	}{
		// repository and tag.
		{
			name: "repository without a tag key",
			values: `
image:
  repository: rancher/a
`,
			want: []string{`values.yaml image: "repository" is set but "tag" is empty`},
		},
		{
			name: "repository with an empty tag",
			values: `
image:
  repository: rancher/a
  tag: ""
`,
			want: []string{`values.yaml image: "repository" is set but "tag" is empty`},
		},
		{
			name: "repository with a null tag",
			values: `
image:
  repository: rancher/a
  tag:
`,
			want: []string{`values.yaml image: "repository" is set but "tag" is empty`},
		},
		{
			name: "repository with a blank tag",
			values: `
image:
  repository: rancher/a
  tag: "  "
`,
			want: []string{`values.yaml image: "repository" is set but "tag" is empty`},
		},
		{
			name: "tag without a repository",
			values: `
image:
  repository: ""
  tag: v1
`,
			want: []string{`values.yaml image: "tag" is set but "repository" is empty`},
		},
		{
			name: "tag with a null repository",
			values: `
image:
  repository:
  tag: v1
`,
			want: []string{`values.yaml image: "tag" is set but "repository" is empty`},
		},

		// Types.
		{
			name: "tag written as a number",
			values: `
image:
  repository: rancher/a
  tag: 1.10
`,
			want: []string{`values.yaml image: "repository" and "tag" must be strings`},
		},
		{
			name: "tag written as an integer",
			values: `
image:
  repository: rancher/a
  tag: 3
`,
			want: []string{`values.yaml image: "repository" and "tag" must be strings`},
		},
		{
			name: "repository written as a number",
			values: `
image:
  repository: 12
  tag: v1
`,
			want: []string{`values.yaml image: "repository" and "tag" must be strings`},
		},
		{
			name: "repository written as a list",
			values: `
image:
  repository: [rancher/a]
  tag: v1
`,
			want: []string{`values.yaml image: "repository" and "tag" must be strings`},
		},
		{
			name: "a tag that is a number beside an empty repository",
			values: `
image:
  repository: ""
  tag: 2
`,
			want: []string{`values.yaml image: "repository" and "tag" must be strings`},
		},

		// Where the path points.
		{
			name: "problem at the top level",
			values: `
repository: rancher/a
`,
			want: []string{`values.yaml .: "repository" is set but "tag" is empty`},
		},
		{
			name: "problem inside a list",
			values: `
containers:
  - image:
      repository: rancher/a
  - image:
      repository: rancher/b
      tag: "1"
  - image:
      repository: rancher/c
`,
			want: []string{
				`values.yaml containers[0].image: "repository" is set but "tag" is empty`,
				`values.yaml containers[2].image: "repository" is set but "tag" is empty`,
			},
		},
		{
			name: "problem deep in the tree",
			values: `
a:
  b:
    c:
      image:
        repository: rancher/a
`,
			want: []string{`values.yaml a.b.c.image: "repository" is set but "tag" is empty`},
		},

		// A limit of the rule: any map with a repository key is read as an image.
		{
			name: "a chart repository URL under a repository key is read as an image",
			values: `
sources:
  - repository: https://example.com/charts
`,
			want: []string{`values.yaml sources[0]: "repository" is set but "tag" is empty`},
		},

		// Several problems, reported in key order.
		{
			name: "several problems",
			values: `
zeta:
  image:
    repository: rancher/z
alpha:
  image:
    repository: rancher/a
middle:
  image:
    repository: ""
    tag: v1
`,
			want: []string{
				`values.yaml alpha.image: "repository" is set but "tag" is empty`,
				`values.yaml middle.image: "tag" is set but "repository" is empty`,
				`values.yaml zeta.image: "repository" is set but "tag" is empty`,
			},
		},
		{
			name: "an image inside an image block",
			values: `
image:
  repository: rancher/a
  sidecar:
    image:
      repository: rancher/b
`,
			want: []string{
				`values.yaml image: "repository" is set but "tag" is empty`,
				`values.yaml image.sidecar.image: "repository" is set but "tag" is empty`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, checkValuesYaml(parseValues(t, tt.values)))
		})
	}
}
