package index

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	chart "helm.sh/helm/v4/pkg/chart/v2"
	repo "helm.sh/helm/v4/pkg/repo/v1"

	"github.com/rancher/cargueiro/internal/rules"
)

func TestCheckIcon(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"assets/logos/fleet.svg":      "<svg/>",
		"assets/logos/fleet.png":      "png",
		"assets/logos/noextension":    "logo",
		"assets/other/fleet.svg":      "<svg/>",
		"assets/logos/folder/inside":  "logo",
		"assets/logos/target-of-link": "logo",
	})
	require.NoError(t, os.Symlink("target-of-link", filepath.Join(dir, "assets", "logos", "link.svg")))

	main := rules.Chart{Name: "fleet", Kind: rules.KindMain}

	tests := []struct {
		name    string
		rule    rules.Chart
		icon    string
		wantErr string
	}{
		{
			name: "a logo that exists",
			rule: main,
			icon: "file://assets/logos/fleet.svg",
		},
		{
			name: "whatever the extension of the logo",
			rule: main,
			icon: "file://assets/logos/fleet.png",
		},
		{
			name: "a logo with no extension",
			rule: main,
			icon: "file://assets/logos/noextension",
		},
		{
			name: "a crd chart is exempt, with no icon",
			rule: rules.Chart{Name: "fleet-crd", Kind: rules.KindCRD},
			icon: "",
		},
		{
			name: "an extra chart is exempt, with an icon that is not local",
			rule: rules.Chart{Name: "fleet-agent", Kind: rules.KindExtra},
			icon: "https://example.com/fleet.svg",
		},
		{
			name: "a main chart marked icon: skip is exempt",
			rule: rules.Chart{Name: "fleet", Kind: rules.KindMain, SkipIcon: true},
			icon: "",
		},
		{
			name:    "no icon",
			rule:    main,
			icon:    "",
			wantErr: `fleet 1 has the icon "", but it must be file://assets/logos/<file>`,
		},
		{
			name:    "an icon on the web",
			rule:    main,
			icon:    "https://charts.rancher.io/assets/logos/fleet.svg",
			wantErr: "but it must be file://assets/logos/<file>",
		},
		{
			name:    "a path with no scheme",
			rule:    main,
			icon:    "assets/logos/fleet.svg",
			wantErr: "but it must be file://assets/logos/<file>",
		},
		{
			name:    "a logo of another folder",
			rule:    main,
			icon:    "file://assets/other/fleet.svg",
			wantErr: "but it must be file://assets/logos/<file>",
		},
		{
			name:    "no file name",
			rule:    main,
			icon:    "file://assets/logos/",
			wantErr: "but it must be file://assets/logos/<file>",
		},
		{
			name:    "a file in a folder of the logos",
			rule:    main,
			icon:    "file://assets/logos/folder/inside",
			wantErr: "but it must be file://assets/logos/<file>",
		},
		{
			name:    "a path that leaves the logos",
			rule:    main,
			icon:    "file://assets/logos/../other/fleet.svg",
			wantErr: "but it must be file://assets/logos/<file>",
		},
		{
			name:    "a logo that does not exist",
			rule:    main,
			icon:    "file://assets/logos/missing.svg",
			wantErr: "but assets/logos/missing.svg does not exist",
		},
		{
			name:    "a folder in place of the logo",
			rule:    main,
			icon:    "file://assets/logos/folder",
			wantErr: "but assets/logos/folder is not a regular file",
		},
		{
			name:    "a link in place of the logo",
			rule:    main,
			icon:    "file://assets/logos/link.svg",
			wantErr: "but assets/logos/link.svg is not a regular file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := &repo.ChartVersion{
				Metadata: &chart.Metadata{Name: tt.rule.Name, Version: "1", Icon: tt.icon},
			}

			err := checkIcon(dir, tt.rule, entry)

			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
