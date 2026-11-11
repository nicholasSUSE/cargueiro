package ship

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// writeFiles creates the given files, with the given content, under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
}

func fleetShipment() Shipment {
	return Shipment{
		Chart:   "fleet",
		Version: "110.0.3+up0.16.3",
		Entry:   []byte("name: fleet\n"),
		Logo:    &Logo{Data: []byte("<svg/>"), Extension: ".svg"},
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestWrite(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, fleetShipment().Write(dir))

	require.Equal(t, "name: fleet\n", readFile(t, filepath.Join(dir, "entries", "fleet", "110.0.3+up0.16.3.yaml")))
	require.Equal(t, "<svg/>", readFile(t, filepath.Join(dir, "assets", "logos", "fleet.svg")))
}

func TestWriteWithNoLogo(t *testing.T) {
	dir := t.TempDir()
	shipment := fleetShipment()
	shipment.Logo = nil

	require.NoError(t, shipment.Write(dir))

	require.FileExists(t, filepath.Join(dir, "entries", "fleet", "110.0.3+up0.16.3.yaml"))
	require.NoDirExists(t, filepath.Join(dir, "assets"))
}

func TestWriteKeepsWhatIsInTheRepository(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"entries/fleet/110.0.2+up0.16.2.yaml": "old entry",
		"assets/logos/other.svg":              "other logo",
		"index.yaml":                          "index",
	})

	require.NoError(t, fleetShipment().Write(dir))

	require.Equal(t, "old entry", readFile(t, filepath.Join(dir, "entries", "fleet", "110.0.2+up0.16.2.yaml")))
	require.Equal(t, "other logo", readFile(t, filepath.Join(dir, "assets", "logos", "other.svg")))
	require.Equal(t, "index", readFile(t, filepath.Join(dir, "index.yaml")))
}

func TestWriteRefuses(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		change  func(*Shipment)
		wantErr string
		// stays are the files that must be as they were.
		stays map[string]string
	}{
		{
			name:    "an entry that is there already",
			files:   map[string]string{"entries/fleet/110.0.3+up0.16.3.yaml": "released"},
			wantErr: "entries/fleet/110.0.3+up0.16.3.yaml already exists: an entry is never replaced",
			stays:   map[string]string{"entries/fleet/110.0.3+up0.16.3.yaml": "released"},
		},
		{
			name:    "an identical entry is refused as well",
			files:   map[string]string{"entries/fleet/110.0.3+up0.16.3.yaml": "name: fleet\n"},
			wantErr: "an entry is never replaced",
			stays:   map[string]string{"entries/fleet/110.0.3+up0.16.3.yaml": "name: fleet\n"},
		},
		{
			name:    "a logo of the chart that is there already",
			files:   map[string]string{"assets/logos/fleet.svg": "the logo"},
			wantErr: "assets/logos/fleet.svg is the logo of fleet already",
			stays:   map[string]string{"assets/logos/fleet.svg": "the logo"},
		},
		{
			name:    "a logo of the chart with another extension",
			files:   map[string]string{"assets/logos/fleet.png": "the logo"},
			wantErr: "assets/logos/fleet.png is the logo of fleet already",
			stays:   map[string]string{"assets/logos/fleet.png": "the logo"},
		},
		{
			name:    "a folder where the logos go is a file",
			files:   map[string]string{"assets/logos": "not a folder"},
			wantErr: "logos",
			stays:   map[string]string{"assets/logos": "not a folder"},
		},
		{
			name:    "no name",
			change:  func(s *Shipment) { s.Chart = "" },
			wantErr: `the chart name "" is not valid`,
		},
		{
			name:    "a name that leaves the repository",
			change:  func(s *Shipment) { s.Chart = "../fleet" },
			wantErr: `the chart name "../fleet" is not valid`,
		},
		{
			name:    "a version with a slash",
			change:  func(s *Shipment) { s.Version = "1/2" },
			wantErr: `the version "1/2" is not valid`,
		},
		{
			name:    "a hidden version",
			change:  func(s *Shipment) { s.Version = ".hidden" },
			wantErr: `the version ".hidden" is not valid`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tt.files)
			shipment := fleetShipment()
			if tt.change != nil {
				tt.change(&shipment)
			}

			err := shipment.Write(dir)

			require.ErrorContains(t, err, tt.wantErr)
			for name, content := range tt.stays {
				require.Equal(t, content, readFile(t, filepath.Join(dir, name)))
			}
			const entry = "entries/fleet/110.0.3+up0.16.3.yaml"
			if _, there := tt.files[entry]; !there {
				require.NoFileExists(t, filepath.Join(dir, entry), "nothing is left of the entry")
			}
		})
	}
}

func TestWriteTakesTheEntryBackWhenTheLogoFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("a folder cannot be made read-only for root")
	}

	dir := t.TempDir()
	// The logos can be looked at but not written to, so the entry, which is
	// written first, has to be taken back.
	logos := filepath.Join(dir, "assets", "logos")
	require.NoError(t, os.MkdirAll(logos, 0o750))
	require.NoError(t, os.Chmod(logos, 0o500))
	t.Cleanup(func() { _ = os.Chmod(logos, 0o700) })

	err := fleetShipment().Write(dir)

	require.Error(t, err)
	require.NoDirExists(t, filepath.Join(dir, "entries"), "the entry and its folders are taken back")
}

func TestWriteNeedsARepository(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))

	tests := []struct {
		name    string
		dir     string
		wantErr string
	}{
		{
			name:    "a folder that is not there",
			dir:     filepath.Join(dir, "missing"),
			wantErr: "is not a folder",
		},
		{
			name:    "a file",
			dir:     file,
			wantErr: "is not a folder",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := fleetShipment().Write(tt.dir)

			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestHasLogo(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"assets/logos/fleet.svg":       "x",
		"assets/logos/longhorn.png":    "x",
		"assets/logos/fleet-crd.svg":   "x",
		"assets/logos/fleet-agent.gif": "x",
	})
	require.NoError(t, os.Mkdir(filepath.Join(dir, "assets", "logos", "harvester.d"), 0o750))

	tests := []struct {
		name  string
		chart string
		want  bool
	}{
		{name: "a logo", chart: "fleet", want: true},
		{name: "a logo with another extension", chart: "longhorn", want: true},
		{name: "a chart that has none", chart: "rancher-backup", want: false},
		{name: "a chart whose name starts as another's", chart: "fleet-c", want: false},
		{name: "a folder is not a logo", chart: "harvester", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HasLogo(dir, tt.chart)

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestHasLogoWithNoLogosFolder(t *testing.T) {
	got, err := HasLogo(t.TempDir(), "fleet")

	require.NoError(t, err)
	require.False(t, got)
}

func TestShipmentPaths(t *testing.T) {
	tests := []struct {
		name     string
		shipment Shipment
		want     []string
	}{
		{
			name:     "the entry and the logo",
			shipment: fleetShipment(),
			want:     []string{"entries/fleet/110.0.3+up0.16.3.yaml", "assets/logos/fleet.svg"},
		},
		{
			name: "the entry only",
			shipment: Shipment{
				Chart:   "fleet",
				Version: "110.0.3+up0.16.3",
				Entry:   []byte("x"),
			},
			want: []string{"entries/fleet/110.0.3+up0.16.3.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.shipment.Paths())
		})
	}
}

func familyShipments() []Shipment {
	crd := Shipment{Chart: "fleet-crd", Version: "110.0.3+up0.16.3", Entry: []byte("name: fleet-crd\n")}
	agent := Shipment{Chart: "fleet-agent", Version: "110.0.3+up0.16.3", Entry: []byte("name: fleet-agent\n")}
	return []Shipment{fleetShipment(), crd, agent}
}

func TestWriteAll(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, WriteAll(dir, familyShipments()))

	for _, chart := range []string{"fleet", "fleet-crd", "fleet-agent"} {
		require.Equal(t, "name: "+chart+"\n", readFile(t, filepath.Join(dir, "entries", chart, "110.0.3+up0.16.3.yaml")))
	}
	require.Equal(t, "<svg/>", readFile(t, filepath.Join(dir, "assets", "logos", "fleet.svg")))
}

func TestWriteAllWritesNothingWhenOneCannotBeWritten(t *testing.T) {
	tests := []struct {
		name    string
		files   map[string]string
		wantErr string
	}{
		{
			name:    "an entry is there already",
			files:   map[string]string{"entries/fleet-agent/110.0.3+up0.16.3.yaml": "old\n"},
			wantErr: "already exists",
		},
		{
			name:    "a chart has its logo already",
			files:   map[string]string{"assets/logos/fleet.png": "old"},
			wantErr: "is the logo of fleet already",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFiles(t, dir, tt.files)

			err := WriteAll(dir, familyShipments())

			require.ErrorContains(t, err, tt.wantErr)
			require.NoFileExists(t, filepath.Join(dir, "entries", "fleet", "110.0.3+up0.16.3.yaml"))
			require.NoFileExists(t, filepath.Join(dir, "entries", "fleet-crd", "110.0.3+up0.16.3.yaml"))
			require.NoFileExists(t, filepath.Join(dir, "assets", "logos", "fleet.svg"))
		})
	}
}

func TestWriteAllTakesBackWhatItWroteWhenALaterOneFails(t *testing.T) {
	dir := t.TempDir()
	// entries/fleet-agent is a file, so its folder cannot be made.
	writeFiles(t, dir, map[string]string{"entries/fleet-agent": "in the way"})

	err := WriteAll(dir, familyShipments())

	require.Error(t, err)
	require.NoFileExists(t, filepath.Join(dir, "entries", "fleet", "110.0.3+up0.16.3.yaml"))
	require.NoFileExists(t, filepath.Join(dir, "entries", "fleet-crd", "110.0.3+up0.16.3.yaml"))
	require.NoFileExists(t, filepath.Join(dir, "assets", "logos", "fleet.svg"))
	require.NoDirExists(t, filepath.Join(dir, "entries", "fleet"), "the folders it made are gone too")
	require.Equal(t, "in the way", readFile(t, filepath.Join(dir, "entries", "fleet-agent")))
}

func TestWriteAllWithNothingToWrite(t *testing.T) {
	require.NoError(t, WriteAll(t.TempDir(), nil))
}
