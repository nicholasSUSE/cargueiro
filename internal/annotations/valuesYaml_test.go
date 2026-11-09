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

// valuesProblems returns the problems scanValues finds in values.
func valuesProblems(values map[string]any) []string {
	_, problems := scanValues(values)
	return problems
}

func TestScanValuesValid(t *testing.T) {
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
			require.Empty(t, valuesProblems(parseValues(t, tt.values)))
		})
	}

	require.Empty(t, valuesProblems(nil))
	require.Empty(t, valuesProblems(map[string]any{}))
}

func TestScanValuesInvalid(t *testing.T) {
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
			name: "repository with the tag latest",
			values: `
image:
  repository: rancher/a
  tag: latest
`,
			want: []string{`values.yaml image: "tag" must not be "latest"`},
		},
		{
			name: "a nested image with the tag latest",
			values: `
image:
  operator:
    repository: rancher/a
    tag: v1
  scan:
    repository: rancher/b
    tag: latest
`,
			want: []string{`values.yaml image.scan: "tag" must not be "latest"`},
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

		// Mappings with a key that is not text.
		{
			name: "image beside a numeric key",
			values: `
sidecars:
  200: x
  image:
    repository: rancher/a
`,
			want: []string{`values.yaml sidecars.image: "repository" is set but "tag" is empty`},
		},
		{
			name: "image under a numeric key",
			values: `
ports:
  8080:
    image:
      repository: rancher/a
`,
			want: []string{`values.yaml ports.8080.image: "repository" is set but "tag" is empty`},
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
			require.Equal(t, tt.want, valuesProblems(parseValues(t, tt.values)))
		})
	}
}

func TestScanValuesImages(t *testing.T) {
	tests := []struct {
		name   string
		values string
		want   []valuesImage
	}{
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
			want: []valuesImage{
				{Path: "agentImage", Image: "rancher/fleet-agent:v0.16.3"},
				{Path: "image", Image: "rancher/fleet:v0.16.3"},
			},
		},
		{
			name: "optional keys are ignored",
			values: `
image:
  registry: stgregistry.suse.com
  repository: rancher/a
  tag: "1"
  sha: abc
  hash: def
  imagePullPolicy: Always
`,
			want: []valuesImage{{Path: "image", Image: "rancher/a:1"}},
		},
		{
			name: "paths through maps and lists",
			values: `
containers:
  - image:
      repository: rancher/a
      tag: "1"
  - image:
      repository: rancher/b
      tag: "2"
`,
			want: []valuesImage{
				{Path: "containers[0].image", Image: "rancher/a:1"},
				{Path: "containers[1].image", Image: "rancher/b:2"},
			},
		},
		{
			name: "the same image twice is found twice",
			values: `
a:
  repository: rancher/x
  tag: "1"
b:
  repository: rancher/x
  tag: "1"
`,
			want: []valuesImage{
				{Path: "a", Image: "rancher/x:1"},
				{Path: "b", Image: "rancher/x:1"},
			},
		},
		{
			name: "blocks with no image configured and broken blocks are not images",
			values: `
none:
  repository: ""
  tag: ""
broken:
  repository: rancher/a
flat:
  image: rancher/b:1
`,
		},
		{
			name: "images beside and under numeric keys",
			values: `
sidecars:
  200: x
  image:
    repository: rancher/a
    tag: "1"
ports:
  8080:
    image:
      repository: rancher/b
      tag: "2"
`,
			want: []valuesImage{
				{Path: "ports.8080.image", Image: "rancher/b:2"},
				{Path: "sidecars.image", Image: "rancher/a:1"},
			},
		},
		{
			name: "top level image",
			values: `
repository: rancher/a
tag: "1"
`,
			want: []valuesImage{{Path: "", Image: "rancher/a:1"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			images, _ := scanValues(parseValues(t, tt.values))
			require.Equal(t, tt.want, images)
		})
	}
}
