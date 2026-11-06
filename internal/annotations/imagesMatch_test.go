package annotations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckDependenciesMatch(t *testing.T) {
	tests := []struct {
		name       string
		annotation string
		values     string
		want       []string
	}{
		// The two sides agree.
		{
			name:       "an image in both",
			annotation: "- name: a\n  image: rancher/a:1\n",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
		},
		{
			name:       "fleet",
			annotation: "- name: fleet\n  image: rancher/fleet:v0.16.3\n- name: fleet-agent\n  image: rancher/fleet-agent:v0.16.3\n",
			values:     "image:\n  repository: rancher/fleet\n  tag: v0.16.3\nagentImage:\n  repository: rancher/fleet-agent\n  tag: v0.16.3\n",
		},
		{
			name:       "the order does not matter",
			annotation: "- name: b\n  image: rancher/b:2\n- name: a\n  image: rancher/a:1\n",
			values:     "a:\n  repository: rancher/a\n  tag: \"1\"\nb:\n  repository: rancher/b\n  tag: \"2\"\n",
		},
		{
			name:       "the same image in values twice needs one entry",
			annotation: "- name: x\n  image: rancher/x:1\n",
			values:     "a:\n  repository: rancher/x\n  tag: \"1\"\nb:\n  repository: rancher/x\n  tag: \"1\"\n",
		},
		{
			name:       "the registry in values is not part of the comparison",
			annotation: "- name: a\n  image: rancher/a:1\n",
			values:     "image:\n  registry: stgregistry.suse.com\n  repository: rancher/a\n  tag: \"1\"\n",
		},

		// Nothing to compare.
		{
			name: "no annotation and no images",
		},
		{
			name:   "no annotation and values without images",
			values: "replicas: 2\n",
		},
		{
			name:   "a flat image string is not an image, so nothing is expected",
			values: "testReceiver:\n  image: fluent/fluent-bit\n",
		},
		{
			name:   "an image block with nothing configured",
			values: "image:\n  repository: \"\"\n  tag: \"\"\n",
		},

		// values.yaml -> annotation.
		{
			name:   "no annotation",
			values: "image:\n  repository: rancher/a\n  tag: \"1\"\n",
			want:   []string{`values.yaml image: image "rancher/a:1" is not listed in helm.sh/images`},
		},
		{
			name:       "a blank annotation",
			annotation: "   ",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
			want:       []string{`values.yaml image: image "rancher/a:1" is not listed in helm.sh/images`},
		},
		{
			name:       "an empty list",
			annotation: "[]",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
			want:       []string{`values.yaml image: image "rancher/a:1" is not listed in helm.sh/images`},
		},
		{
			name:       "another tag is another image",
			annotation: "- name: a\n  image: rancher/a:2\n",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
			want: []string{
				`values.yaml image: image "rancher/a:1" is not listed in helm.sh/images`,
				`helm.sh/images entry 1: image "rancher/a:2" is not in values.yaml and has no dependency`,
			},
		},
		{
			name:       "one of two images missing",
			annotation: "- name: a\n  image: rancher/a:1\n",
			values:     "a:\n  repository: rancher/a\n  tag: \"1\"\nb:\n  repository: rancher/b\n  tag: \"2\"\n",
			want:       []string{`values.yaml b: image "rancher/b:2" is not listed in helm.sh/images`},
		},
		{
			name:       "the same missing image is reported once, at its first path",
			annotation: "- name: a\n  image: rancher/a:1\n",
			values:     "z:\n  repository: rancher/x\n  tag: \"1\"\ny:\n  repository: rancher/x\n  tag: \"1\"\n",
			want: []string{
				`values.yaml y: image "rancher/x:1" is not listed in helm.sh/images`,
				`helm.sh/images entry 1: image "rancher/a:1" is not in values.yaml and has no dependency`,
			},
		},
		{
			name:       "a registry on the annotation image makes it a different image",
			annotation: "- name: a\n  image: registry.suse.com/rancher/a:1\n",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
			want: []string{
				`values.yaml image: image "rancher/a:1" is not listed in helm.sh/images`,
				`helm.sh/images entry 1: image "registry.suse.com/rancher/a:1" is not in values.yaml and has no dependency`,
			},
		},

		// annotation -> values.
		{
			name:       "an entry that is not in values and has no dependency",
			annotation: "- name: a\n  image: rancher/a:1\n",
			values:     "replicas: 2\n",
			want:       []string{`helm.sh/images entry 1: image "rancher/a:1" is not in values.yaml and has no dependency`},
		},
		{
			name:       "an entry that is not in values but has a dependency",
			annotation: "- name: a\n  image: rancher/a:1\n  dependency: grafana\n",
			values:     "replicas: 2\n",
		},
		{
			name:       "the value of the dependency is not checked",
			annotation: "- name: a\n  image: rancher/a:1\n  dependency: whatever\n",
			values:     "replicas: 2\n",
		},
		{
			name:       "an empty dependency does not count",
			annotation: "- name: a\n  image: rancher/a:1\n  dependency: \"\"\n",
			values:     "replicas: 2\n",
			want:       []string{`helm.sh/images entry 1: image "rancher/a:1" is not in values.yaml and has no dependency`},
		},
		{
			name:       "a blank dependency does not count",
			annotation: "- name: a\n  image: rancher/a:1\n  dependency: \"  \"\n",
			values:     "replicas: 2\n",
			want:       []string{`helm.sh/images entry 1: image "rancher/a:1" is not in values.yaml and has no dependency`},
		},
		{
			name:       "a dependency on an entry that is in values changes nothing",
			annotation: "- name: a\n  image: rancher/a:1\n  dependency: grafana\n",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
		},
		{
			name:       "several entries, only the unbacked one without a dependency fails",
			annotation: "- name: a\n  image: rancher/a:1\n- name: b\n  image: rancher/b:2\n  dependency: sub\n- name: c\n  image: rancher/c:3\n",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
			want:       []string{`helm.sh/images entry 3: image "rancher/c:3" is not in values.yaml and has no dependency`},
		},
		{
			name:       "entry numbers follow the annotation",
			annotation: "- name: a\n  image: rancher/a:1\n  dependency: x\n- name: b\n  image: rancher/b:2\n",
			values:     "replicas: 2\n",
			want:       []string{`helm.sh/images entry 2: image "rancher/b:2" is not in values.yaml and has no dependency`},
		},

		// A flat image string in values.yaml.
		{
			name:       "an untagged entry for a flat image: left to the format check",
			annotation: "- name: fluent-bit\n  image: fluent/fluent-bit\n",
			values:     "testReceiver:\n  image: fluent/fluent-bit\n",
		},
		{
			name:       "a well-formed entry for a flat image is not backed by values",
			annotation: "- name: fluent-bit\n  image: fluent/fluent-bit:1.9\n",
			values:     "testReceiver:\n  image: fluent/fluent-bit\n",
			want:       []string{`helm.sh/images entry 1: image "fluent/fluent-bit:1.9" is not in values.yaml and has no dependency`},
		},
		{
			name:       "the same entry with a dependency",
			annotation: "- name: fluent-bit\n  image: fluent/fluent-bit:1.9\n  dependency: logging\n",
			values:     "testReceiver:\n  image: fluent/fluent-bit\n",
		},

		// Badly formed entries are left to the format check.
		{
			name:       "an entry without a name is not reported again",
			annotation: "- image: rancher/a:1\n",
			values:     "replicas: 2\n",
		},
		{
			name:       "an entry whose name is already used is not reported again",
			annotation: "- name: a\n  image: rancher/a:1\n  dependency: x\n- name: a\n  image: rancher/b:2\n",
			values:     "replicas: 2\n",
		},
		{
			name:       "a null item is not reported again",
			annotation: "- \n- name: a\n  image: rancher/a:1\n  dependency: x\n",
			values:     "replicas: 2\n",
		},
		{
			name:       "an entry without a name still lists its image for values",
			annotation: "- image: rancher/a:1\n",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
		},
		{
			name:       "an entry without a tag is not reported again",
			annotation: "- name: a\n  image: rancher/a\n",
			values:     "replicas: 2\n",
		},
		{
			name:       "an entry without an image is not reported again",
			annotation: "- name: a\n",
			values:     "replicas: 2\n",
		},
		{
			name:       "an annotation that cannot be read is not reported again",
			annotation: "not a list",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
		},
		{
			name:       "an annotation that is not YAML is not reported again",
			annotation: "- name: [",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
		},

		// Surrounding space.
		{
			name:       "space around the entry image is ignored",
			annotation: "- name: a\n  image: \" rancher/a:1 \"\n",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
		},

		// Both rules at once, the values side first.
		{
			name:       "both directions",
			annotation: "- name: x\n  image: rancher/x:9\n",
			values:     "image:\n  repository: rancher/a\n  tag: \"1\"\n",
			want: []string{
				`values.yaml image: image "rancher/a:1" is not listed in helm.sh/images`,
				`helm.sh/images entry 1: image "rancher/x:9" is not in values.yaml and has no dependency`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			annotations := map[string]string{}
			if tt.annotation != "" {
				annotations["helm.sh/images"] = tt.annotation
			}
			images, _ := scanValues(parseValues(t, tt.values))
			require.Equal(t, tt.want, checkDependenciesMatch(annotations, images))
		})
	}

	require.Empty(t, checkDependenciesMatch(nil, nil))
}
