package annotations

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCheckDependenciesAnnotationStandardValid(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		// What the released charts carry.
		{
			name:  "fleet",
			value: "- name: fleet\n  image: rancher/fleet:v0.16.3\n- name: fleet-agent\n  image: rancher/fleet-agent:v0.16.3\n",
		},
		{
			name:  "fleet-agent",
			value: "- name: fleet-agent\n  image: rancher/fleet-agent:v0.16.3\n",
		},

		// The example in HIP-15.
		{
			name:  "HIP-15 podinfo example",
			value: "- name: podinfo\n  image: ghcr.io/stefanprodan/podinfo:6.0.0\n- name: redis\n  image: dockerhub.io/_/redis:6.0.8\n  whitelisted: true\n",
		},

		// The list.
		{
			name:  "empty list",
			value: "[]",
		},
		{
			name:  "one entry",
			value: "- name: a\n  image: rancher/a:1\n",
		},
		{
			name:  "many entries",
			value: manyEntries(50),
		},
		{
			name:  "flow style",
			value: `[{name: a, image: "rancher/a:1"}, {name: b, image: "rancher/b:2"}]`,
		},
		{
			name:  "comments",
			value: "# images\n- name: a # the first\n  image: rancher/a:1\n",
		},
		{
			name:  "windows line endings",
			value: "- name: a\r\n  image: rancher/a:1\r\n",
		},
		{
			name:  "keys in the other order",
			value: "- image: rancher/a:1\n  name: a\n",
		},

		// Entries.
		{
			name:  "extra properties are allowed",
			value: "- name: a\n  image: rancher/a:1\n  whitelisted: true\n  publicKey: abc\n",
		},
		{
			name:  "dependency",
			value: "- name: a\n  image: rancher/a:1\n  dependency: sub\n",
		},
		{
			name:  "the same image under different names",
			value: "- name: a\n  image: rancher/x:1\n- name: b\n  image: rancher/x:1\n",
		},
		{
			name:  "names differing in case are different",
			value: "- name: A\n  image: rancher/a:1\n- name: a\n  image: rancher/b:1\n",
		},
		{
			name:  "a null entry is skipped by the YAML decoder",
			value: "- \n- name: a\n  image: rancher/a:1\n",
		},
		{
			name:  "name with spaces inside",
			value: "- name: my image\n  image: rancher/a:1\n",
		},
		{
			name:  "surrounding space in the image is ignored",
			value: "- name: a\n  image: \" rancher/a:1 \"\n",
		},

		// Images.
		{
			name:  "no registry",
			value: "- name: a\n  image: rancher/a:1\n",
		},
		{
			name:  "registry",
			value: "- name: a\n  image: registry.suse.com/rancher/a:1\n",
		},
		{
			name:  "registry with a port",
			value: "- name: a\n  image: localhost:5000/rancher/a:1\n",
		},
		{
			name:  "deep path",
			value: "- name: a\n  image: rancher/elemental-channel/sl-micro:6.2-base\n",
		},
		{
			name:  "single segment",
			value: "- name: a\n  image: busybox:1.36\n",
		},
		{
			name:  "tag with a v",
			value: "- name: a\n  image: rancher/a:v1.2.3\n",
		},
		{
			name:  "tag with a build suffix",
			value: "- name: a\n  image: rancher/a:0.30.0-12.16\n",
		},
		{
			name:  "tag with a prerelease",
			value: "- name: a\n  image: rancher/a:v1.16-4.10-full\n",
		},
		{
			name:  "tag with underscore",
			value: "- name: a\n  image: rancher/a:110.0.3_up0.16.3\n",
		},
		{
			name:  "tag latest",
			value: "- name: a\n  image: rancher/a:latest\n",
		},

		// Not this check's concern.
		{
			name:  "absent",
			value: "",
		},
		{
			name:  "blank",
			value: "  \n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Empty(t, checkDependenciesAnnotationStandard(map[string]string{"helm.sh/images": tt.value}))
		})
	}

	require.Empty(t, checkDependenciesAnnotationStandard(nil))
	require.Empty(t, checkDependenciesAnnotationStandard(map[string]string{}))
}

func TestCheckDependenciesAnnotationStandardInvalid(t *testing.T) {
	const (
		key      = "helm.sh/images"
		notAList = key + `: must be a YAML list of entries with a name and an image`
		noName   = key + " entry %d: name must be a non-empty string"
		noImage  = key + " entry %d: image must be a non-empty string"
	)
	entryProblem := func(format string, n int) string { return fmt.Sprintf(format, n) }

	tests := []struct {
		name  string
		value string
		want  []string
	}{
		// Not a list of entries.
		{
			name:  "text",
			value: "fleet",
			want:  []string{notAList},
		},
		{
			name:  "number",
			value: "12",
			want:  []string{notAList},
		},
		{
			name:  "boolean",
			value: "true",
			want:  []string{notAList},
		},
		{
			name:  "a map",
			value: "name: fleet\nimage: rancher/fleet:v1\n",
			want:  []string{notAList},
		},
		{
			name:  "null",
			value: "null",
			want:  []string{notAList},
		},
		{
			name:  "list of strings",
			value: "- rancher/fleet:v1\n",
			want:  []string{notAList},
		},
		{
			name:  "list of lists",
			value: "- - a\n",
			want:  []string{notAList},
		},
		{
			name:  "image is a list",
			value: "- name: a\n  image: [rancher/a:1]\n",
			want:  []string{notAList},
		},
		{
			name:  "image is a map",
			value: "- name: a\n  image: {repository: rancher/a, tag: \"1\"}\n",
			want:  []string{notAList},
		},

		// Name.
		{
			name:  "name missing",
			value: "- image: rancher/a:1\n",
			want:  []string{entryProblem(noName, 1)},
		},
		{
			name:  "name empty",
			value: "- name: \"\"\n  image: rancher/a:1\n",
			want:  []string{entryProblem(noName, 1)},
		},
		{
			name:  "name blank",
			value: "- name: \"   \"\n  image: rancher/a:1\n",
			want:  []string{entryProblem(noName, 1)},
		},
		{
			name:  "name null",
			value: "- name: null\n  image: rancher/a:1\n",
			want:  []string{entryProblem(noName, 1)},
		},
		{
			name:  "name used twice",
			value: "- name: shell\n  image: rancher/a:1\n- name: x\n  image: rancher/x:1\n- name: shell\n  image: rancher/b:2\n",
			want:  []string{key + ` entry 3: name "shell" is already used by entry 1`},
		},
		{
			name:  "name used twice, ignoring surrounding space",
			value: "- name: a\n  image: rancher/a:1\n- name: \" a\"\n  image: rancher/b:1\n",
			want:  []string{key + ` entry 2: name "a" is already used by entry 1`},
		},
		{
			name:  "name used three times",
			value: "- name: a\n  image: rancher/a:1\n- name: a\n  image: rancher/b:1\n- name: a\n  image: rancher/c:1\n",
			want: []string{
				key + ` entry 2: name "a" is already used by entry 1`,
				key + ` entry 3: name "a" is already used by entry 1`,
			},
		},

		// Image.
		{
			name:  "image missing",
			value: "- name: a\n",
			want:  []string{entryProblem(noImage, 1)},
		},
		{
			name:  "image empty",
			value: "- name: a\n  image: \"\"\n",
			want:  []string{entryProblem(noImage, 1)},
		},
		{
			name:  "image blank",
			value: "- name: a\n  image: \"  \"\n",
			want:  []string{entryProblem(noImage, 1)},
		},
		{
			name:  "image null",
			value: "- name: a\n  image: null\n",
			want:  []string{entryProblem(noImage, 1)},
		},
		{
			name:  "image without a tag",
			value: "- name: a\n  image: rancher/a\n",
			want:  []string{key + ` entry 1: image "rancher/a" must look like "repository:tag"`},
		},
		{
			name:  "image with an empty tag",
			value: "- name: a\n  image: \"rancher/a:\"\n",
			want:  []string{key + ` entry 1: image "rancher/a:" must look like "repository:tag"`},
		},
		{
			name:  "image with a registry port but no tag",
			value: "- name: a\n  image: localhost:5000/rancher/a\n",
			want:  []string{key + ` entry 1: image "localhost:5000/rancher/a" must look like "repository:tag"`},
		},
		{
			name:  "image with no repository",
			value: "- name: a\n  image: \":1\"\n",
			want:  []string{key + ` entry 1: image ":1" must look like "repository:tag"`},
		},
		{
			name:  "image ending with a slash",
			value: "- name: a\n  image: \"rancher/a/\"\n",
			want:  []string{key + ` entry 1: image "rancher/a/" must look like "repository:tag"`},
		},
		{
			name:  "image with a space",
			value: "- name: a\n  image: \"rancher/a: 1\"\n",
			want:  []string{key + ` entry 1: image "rancher/a: 1" must look like "repository:tag" without spaces`},
		},
		{
			name:  "image with a tab",
			value: "- name: a\n  image: \"rancher/a:\\t1\"\n",
			want:  []string{key + " entry 1: image \"rancher/a:\\t1\" must look like \"repository:tag\" without spaces"},
		},

		// Several problems at once, each reported once and in order.
		{
			name:  "nothing in the entry",
			value: "- foo: bar\n",
			want:  []string{entryProblem(noName, 1), entryProblem(noImage, 1)},
		},
		{
			name:  "problems in different entries",
			value: "- name: a\n  image: rancher/a\n- image: rancher/b:1\n- name: a\n  image: rancher/c:1\n",
			want: []string{
				key + ` entry 1: image "rancher/a" must look like "repository:tag"`,
				entryProblem(noName, 2),
				key + ` entry 3: name "a" is already used by entry 1`,
			},
		},
		{
			name:  "name used twice and no tag in the same entry",
			value: "- name: a\n  image: rancher/a:1\n- name: a\n  image: rancher/b\n",
			want: []string{
				key + ` entry 2: name "a" is already used by entry 1`,
				key + ` entry 2: image "rancher/b" must look like "repository:tag"`,
			},
		},

		// The Kafka example of HIP-15 repeats the name "shell". HIP-15 allows
		// that; Rancher policy does not.
		{
			name:  "HIP-15 Kafka example",
			value: "- name: kafka\n  image: docker.io/bitnami/kafka:2.8.1-debian-10-r0\n- name: kubectl\n  image: docker.io/bitnami/kubectl:1.19.5-debian-10-r3\n- name: shell\n  image: docker.io/bitnami/bitnami-shell:10-debian-10-r199\n- name: kafka-exporter\n  image: docker.io/bitnami/kafka-exporter:1.4.2-debian-10-r5\n- name: jmx-exporter\n  image: docker.io/bitnami/bitnami/jmx-exporter:0.16.1-debian-10-r66\n- name: zookeeper\n  image: docker.io/bitnami/zookeeper:3.7.0-debian-10-r157\n  dependency: zookeeper\n- name: shell\n  image: docker.io/bitnami/bitnami-shell:10-debian-10-r202\n  dependency: zookeeper\n",
			want:  []string{key + ` entry 7: name "shell" is already used by entry 3`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkDependenciesAnnotationStandard(map[string]string{"helm.sh/images": tt.value})
			require.Equal(t, tt.want, got)
		})
	}
}

func TestCheckDependenciesAnnotationStandardNotYAML(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "unclosed bracket",
			value: "- name: [",
		},
		{
			name:  "unquoted trailing colon in the image",
			value: "- name: a\n  image: rancher/a:\n",
		},
		{
			name:  "bad indentation",
			value: "- name: a\n image: rancher/a:1\n  name: b\n",
		},
		{
			name:  "tab indentation",
			value: "- name: a\n\timage: rancher/a:1\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkDependenciesAnnotationStandard(map[string]string{"helm.sh/images": tt.value})
			require.Len(t, got, 1)
			require.Contains(t, got[0], "helm.sh/images: must be a YAML list of entries with a name and an image: ")
		})
	}
}

func TestCheckDependenciesAnnotationStandardOtherKeys(t *testing.T) {
	const bad = "fleet"

	tests := []struct {
		name        string
		annotations map[string]string
	}{
		{
			name:        "nil",
			annotations: nil,
		},
		{
			name:        "unrelated annotation",
			annotations: map[string]string{"catalog.cattle.io/certified": "rancher"},
		},
		{
			name:        "under the catalog prefix",
			annotations: map[string]string{"catalog.cattle.io/helm.sh/images": bad},
		},
		{
			name:        "different case",
			annotations: map[string]string{"Helm.sh/images": bad},
		},
		{
			name:        "bare images key, as in the HIP-15 Kafka example",
			annotations: map[string]string{"images": bad},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Empty(t, checkDependenciesAnnotationStandard(tt.annotations))
		})
	}
}

func TestValidateHelmImages(t *testing.T) {
	require.Empty(t, ValidateHelmImages(nil))
	require.Empty(t, ValidateHelmImages(map[string]string{"helm.sh/images": "- name: a\n  image: rancher/a:1\n"}))
	require.Equal(t,
		[]string{"helm.sh/images entry 1: image must be a non-empty string"},
		ValidateHelmImages(map[string]string{"helm.sh/images": "- name: a\n"}))
}

func manyEntries(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "- name: image-%d\n  image: rancher/image-%d:1\n", i, i)
	}
	return b.String()
}
