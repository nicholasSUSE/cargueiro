package chart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

func TestSetVersion(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		version string
		want    string
	}{
		{
			name:    "a plain value",
			in:      "apiVersion: v2\nname: fleet\nversion: 0.16.3\nappVersion: 0.16.3\n",
			version: "110.0.3+up0.16.3",
			want:    "apiVersion: v2\nname: fleet\nversion: 110.0.3+up0.16.3\nappVersion: 0.16.3\n",
		},
		{
			name:    "a double quoted value",
			in:      "name: rancher-webhook\nversion: \"999\"\nappVersion: \"999\"\n",
			version: "111.0.4+up0.11.4",
			want:    "name: rancher-webhook\nversion: 111.0.4+up0.11.4\nappVersion: \"999\"\n",
		},
		{
			name:    "a single quoted value",
			in:      "name: a\nversion: '1.0'\n",
			version: "111.0.0+up1.0.0",
			want:    "name: a\nversion: 111.0.0+up1.0.0\n",
		},
		{
			name:    "a comment after the value is kept",
			in:      "name: a\nversion: 1.0.0   # set by the release\n",
			version: "111.0.0+up1.0.0",
			want:    "name: a\nversion: 111.0.0+up1.0.0   # set by the release\n",
		},
		{
			name:    "the spacing after the colon is kept",
			in:      "name: a\nversion:    1.0.0\n",
			version: "111.0.0+up1.0.0",
			want:    "name: a\nversion:    111.0.0+up1.0.0\n",
		},
		{
			name:    "the first line of the file",
			in:      "version: 1.0.0\nname: a\n",
			version: "111.0.0+up1.0.0",
			want:    "version: 111.0.0+up1.0.0\nname: a\n",
		},
		{
			name:    "the last line, with no newline at the end",
			in:      "name: a\nversion: 1.0.0",
			version: "111.0.0+up1.0.0",
			want:    "name: a\nversion: 111.0.0+up1.0.0",
		},
		{
			name:    "windows line endings are kept",
			in:      "name: a\r\nversion: 1.0.0\r\nappVersion: 1.0.0\r\n",
			version: "111.0.0+up1.0.0",
			want:    "name: a\r\nversion: 111.0.0+up1.0.0\r\nappVersion: 1.0.0\r\n",
		},
		{
			name:    "a version key under annotations is left alone",
			in:      "annotations:\n  version: keep\nname: a\nversion: 1.0.0\n",
			version: "111.0.0+up1.0.0",
			want:    "annotations:\n  version: keep\nname: a\nversion: 111.0.0+up1.0.0\n",
		},
		{
			name:    "appVersion and kubeVersion are left alone",
			in:      "appVersion: 1.0.0\nkubeVersion: '>= 1.30.0-0'\nversion: 1.0.0\n",
			version: "111.0.0+up1.0.0",
			want:    "appVersion: 1.0.0\nkubeVersion: '>= 1.30.0-0'\nversion: 111.0.0+up1.0.0\n",
		},
		{
			name:    "a block with a text after the key is left alone",
			in:      "description: |\n  version: not this one\nversion: 1.0.0\n",
			version: "111.0.0+up1.0.0",
			want:    "description: |\n  version: not this one\nversion: 111.0.0+up1.0.0\n",
		},
		{
			name:    "the same version again",
			in:      "name: a\nversion: 111.0.0+up1.0.0\n",
			version: "111.0.0+up1.0.0",
			want:    "name: a\nversion: 111.0.0+up1.0.0\n",
		},
		{
			name:    "a prerelease",
			in:      "name: a\nversion: 0.7.11-rc.3\n",
			version: "111.0.2+up0.7.11-rc.3",
			want:    "name: a\nversion: 111.0.2+up0.7.11-rc.3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetVersion([]byte(tt.in), tt.version)

			require.NoError(t, err)
			require.Equal(t, tt.want, string(got))
		})
	}
}

func TestSetVersionFails(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		version string
		wantErr string
	}{
		{
			name:    "no version",
			in:      "name: a\nappVersion: 1.0.0\n",
			version: "111.0.0+up1.0.0",
			wantErr: "the Chart.yaml has no version",
		},
		{
			name:    "a version only under annotations",
			in:      "name: a\nannotations:\n  version: x\n",
			version: "111.0.0+up1.0.0",
			wantErr: "the Chart.yaml has no version",
		},
		{
			name:    "empty",
			in:      "",
			version: "111.0.0+up1.0.0",
			wantErr: "the Chart.yaml has no version",
		},
		{
			name:    "two versions",
			in:      "version: 1.0.0\nname: a\nversion: 2.0.0\n",
			version: "111.0.0+up1.0.0",
			wantErr: "the Chart.yaml has more than one version",
		},
		{
			name:    "a block scalar",
			in:      "name: a\nversion: |\n  1.0.0\n",
			version: "111.0.0+up1.0.0",
			wantErr: "is not a value on one line",
		},
		{
			name:    "a value on the next line",
			in:      "name: a\nversion:\n  1.0.0\n",
			version: "111.0.0+up1.0.0",
			wantErr: "is not a value on one line",
		},
		{
			name:    "a version that breaks the YAML",
			in:      "name: a\nversion: 1.0.0\n",
			version: "1: 2",
			wantErr: "would not be valid YAML",
		},
		{
			name:    "a version that reads as something else",
			in:      "name: a\nversion: 1.0.0\n",
			version: "123",
			wantErr: `would not read "123" as the text of the version`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetVersion([]byte(tt.in), tt.version)

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}

// TestSetVersionOnRealCharts changes the version of the charts kept in
// tests/data and checks that the only difference is that line.
func TestSetVersionOnRealCharts(t *testing.T) {
	tests := []struct {
		name string
		path string
		from string
	}{
		{
			name: "the webhook chart as upstream keeps it, with a quoted placeholder",
			path: "rancher-webhook-upstream/src/rancher-webhook/Chart.yaml",
			from: `version: "999"`,
		},
		{
			name: "the compliance chart, with apiVersion v1",
			path: "rancher-compliance-upstream/src/rancher-compliance/Chart.yaml",
			from: "version: 1.5.4",
		},
		{
			name: "a released fleet chart",
			path: "fleet/src/fleet/Chart.yaml",
			from: "version: 110.0.3+up0.16.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "tests", "data", "charts", tt.path))
			require.NoError(t, err)
			original := string(data)
			require.Contains(t, original, "\n"+tt.from+"\n")

			got, err := SetVersion(data, "111.2.3+up9.8.7")

			require.NoError(t, err)
			want := replaceOnce(original, "\n"+tt.from+"\n", "\nversion: 111.2.3+up9.8.7\n")
			require.Equal(t, want, string(got))
		})
	}
}

func replaceOnce(s, old, replacement string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + replacement + s[i+len(old):]
		}
	}
	return s
}

const imagesValue = "- name: fleet\n  image: rancher/fleet:v0.16.3\n- name: fleet-agent\n  image: rancher/fleet-agent:v0.16.3\n"

func TestSetAnnotation(t *testing.T) {
	const block = "  helm.sh/images: |\n    - name: fleet\n      image: rancher/fleet:v0.16.3\n    - name: fleet-agent\n      image: rancher/fleet-agent:v0.16.3\n"

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "added at the end of the annotations",
			in:   "annotations:\n  catalog.cattle.io/certified: rancher\n  catalog.cattle.io/os: linux\napiVersion: v2\nname: a\n",
			want: "annotations:\n  catalog.cattle.io/certified: rancher\n  catalog.cattle.io/os: linux\n" + block + "apiVersion: v2\nname: a\n",
		},
		{
			name: "annotations at the end of the file",
			in:   "apiVersion: v2\nname: a\nannotations:\n  catalog.cattle.io/os: linux\n",
			want: "apiVersion: v2\nname: a\nannotations:\n  catalog.cattle.io/os: linux\n" + block,
		},
		{
			name: "annotations at the end of the file, with no newline",
			in:   "name: a\nannotations:\n  catalog.cattle.io/os: linux",
			want: "name: a\nannotations:\n  catalog.cattle.io/os: linux\n" + block,
		},
		{
			name: "the indentation of the annotations is followed",
			in:   "annotations:\n    catalog.cattle.io/os: linux\nname: a\n",
			want: "annotations:\n    catalog.cattle.io/os: linux\n    helm.sh/images: |\n      - name: fleet\n        image: rancher/fleet:v0.16.3\n      - name: fleet-agent\n        image: rancher/fleet-agent:v0.16.3\nname: a\n",
		},
		{
			name: "a comment and a blank line in the annotations stay",
			in:   "annotations:\n  # the os\n  catalog.cattle.io/os: linux\n\nname: a\n",
			want: "annotations:\n  # the os\n  catalog.cattle.io/os: linux\n" + block + "\nname: a\n",
		},
		{
			name: "the annotation exists as a block and is replaced",
			in:   "annotations:\n  catalog.cattle.io/os: linux\n  helm.sh/images: |\n    - name: old\n      image: rancher/old:v1\n  catalog.cattle.io/certified: rancher\nname: a\n",
			want: "annotations:\n  catalog.cattle.io/os: linux\n" + block + "  catalog.cattle.io/certified: rancher\nname: a\n",
		},
		{
			name: "the annotation exists as a single line and is replaced",
			in:   "annotations:\n  helm.sh/images: '[]'\n  catalog.cattle.io/os: linux\nname: a\n",
			want: "annotations:\n" + block + "  catalog.cattle.io/os: linux\nname: a\n",
		},
		{
			name: "the annotation exists and is the last one",
			in:   "annotations:\n  catalog.cattle.io/os: linux\n  helm.sh/images: |\n    - name: old\n      image: rancher/old:v1\nname: a\n",
			want: "annotations:\n  catalog.cattle.io/os: linux\n" + block + "name: a\n",
		},
		{
			name: "no annotations at all",
			in:   "apiVersion: v2\nname: a\n",
			want: "apiVersion: v2\nname: a\nannotations:\n" + block,
		},
		{
			name: "no annotations, and the file has no newline at the end",
			in:   "name: a",
			want: "name: a\nannotations:\n" + block,
		},
		{
			name: "an empty annotations mapping",
			in:   "annotations: {}\nname: a\n",
			want: "annotations:\n" + block + "name: a\n",
		},
		{
			name: "an annotations key with nothing under it",
			in:   "annotations:\nname: a\n",
			want: "annotations:\n" + block + "name: a\n",
		},
		{
			name: "a helm.sh/images in a description is not the annotation",
			in:   "description: |\n  annotations:\n  helm.sh/images: not this\nannotations:\n  catalog.cattle.io/os: linux\n",
			want: "description: |\n  annotations:\n  helm.sh/images: not this\nannotations:\n  catalog.cattle.io/os: linux\n" + block,
		},
		{
			name: "windows line endings are kept",
			in:   "annotations:\r\n  catalog.cattle.io/os: linux\r\nname: a\r\n",
			want: "annotations:\r\n  catalog.cattle.io/os: linux\r\n  helm.sh/images: |\r\n    - name: fleet\r\n      image: rancher/fleet:v0.16.3\r\n    - name: fleet-agent\r\n      image: rancher/fleet-agent:v0.16.3\r\nname: a\r\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetAnnotation([]byte(tt.in), "helm.sh/images", imagesValue)

			require.NoError(t, err)
			require.Equal(t, tt.want, string(got))
		})
	}
}

func TestSetAnnotationTwiceChangesNothing(t *testing.T) {
	in := "annotations:\n  catalog.cattle.io/os: linux\nname: a\n"

	once, err := SetAnnotation([]byte(in), "helm.sh/images", imagesValue)
	require.NoError(t, err)
	twice, err := SetAnnotation(once, "helm.sh/images", imagesValue)
	require.NoError(t, err)

	require.Equal(t, string(once), string(twice))
}

func TestSetAnnotationValueWithABlankLine(t *testing.T) {
	got, err := SetAnnotation([]byte("annotations:\n  a: b\n"), "note", "first\n\nsecond\n")

	require.NoError(t, err)
	require.Equal(t, "annotations:\n  a: b\n  note: |\n    first\n\n    second\n", string(got))
}

func TestSetAnnotationFails(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		value   string
		wantErr string
	}{
		{
			name:    "no value",
			in:      "name: a\n",
			value:   "",
			wantErr: "an annotation needs a value",
		},
		{
			name:    "a value that starts with a space",
			in:      "name: a\n",
			value:   " x\n",
			wantErr: "cannot start with a space or a new line",
		},
		{
			name:    "annotations twice",
			in:      "annotations:\n  a: b\nannotations:\n  c: d\n",
			value:   "x\n",
			wantErr: "more than one annotations",
		},
		{
			name:    "annotations as a flow mapping with content",
			in:      "annotations: {a: b}\nname: a\n",
			value:   "x\n",
			wantErr: "not a block mapping",
		},
		{
			name:    "the annotation twice",
			in:      "annotations:\n  helm.sh/images: x\n  helm.sh/images: y\n",
			value:   "x\n",
			wantErr: `the annotation "helm.sh/images" is in the Chart.yaml twice`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetAnnotation([]byte(tt.in), "helm.sh/images", tt.value)

			require.ErrorContains(t, err, tt.wantErr)
			require.Nil(t, got)
		})
	}
}

// TestSetAnnotationOnRealCharts adds the annotation to the charts kept in
// tests/data and checks that every line they had is still there, in order, and
// that the annotation reads back.
func TestSetAnnotationOnRealCharts(t *testing.T) {
	paths := []string{
		"rancher-webhook-upstream/src/rancher-webhook/Chart.yaml",
		"rancher-compliance-upstream/src/rancher-compliance/Chart.yaml",
		"fleet/src/fleet/Chart.yaml",
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "tests", "data", "charts", path))
			require.NoError(t, err)

			got, err := SetAnnotation(data, "helm.sh/images", imagesValue)

			require.NoError(t, err)
			require.Contains(t, string(got), "  helm.sh/images: |\n")

			// Take the new lines out and the original must be left, whole.
			var kept []string
			for _, line := range strings.SplitAfter(string(got), "\n") {
				if line != "" {
					kept = append(kept, line)
				}
			}
			var rest string
			for _, line := range kept {
				if line == "  helm.sh/images: |\n" || strings.HasPrefix(line, "    - name: ") || strings.HasPrefix(line, "      image: ") {
					continue
				}
				rest += line
			}
			if !strings.Contains(string(data), "helm.sh/images") {
				require.Equal(t, string(data), rest)
			}

			// The other annotations read back as they were.
			var before, after struct {
				Annotations map[string]string `yaml:"annotations"`
			}
			require.NoError(t, yaml.Unmarshal(data, &before))
			require.NoError(t, yaml.Unmarshal(got, &after))
			delete(before.Annotations, "helm.sh/images")
			require.Equal(t, imagesValue, after.Annotations["helm.sh/images"])
			delete(after.Annotations, "helm.sh/images")
			require.Equal(t, before.Annotations, after.Annotations)
		})
	}
}
