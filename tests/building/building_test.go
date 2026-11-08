//go:build release

// Package building checks the binaries that make release-build writes to bin/.
// It builds nothing itself: run it through make test-release, which builds first.
package building

import (
	"debug/elf"
	"debug/macho"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// binDir is bin/ at the repository root, seen from this package.
func binDir() string {
	return filepath.Join("..", "..", "bin")
}

func binary(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join(binDir(), name)
	info, err := os.Stat(path)
	require.NoError(t, err, "run make test-release, which runs make release-build first")
	require.NotZero(t, info.Size(), "%s is empty", path)
	return path
}

func TestLinuxBuild(t *testing.T) {
	f, err := elf.Open(binary(t, "cargueiro_linux_amd64"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })

	require.Equal(t, elf.EM_X86_64, f.Machine)
	require.Equal(t, elf.ELFOSABI_NONE, f.OSABI)
}

func TestFreeBSDBuild(t *testing.T) {
	f, err := elf.Open(binary(t, "cargueiro_freebsd_amd64"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, f.Close()) })

	require.Equal(t, elf.EM_X86_64, f.Machine)
	require.Equal(t, elf.ELFOSABI_FREEBSD, f.OSABI)
}

func TestDarwinBuilds(t *testing.T) {
	tests := []struct {
		name string
		file string
		cpu  macho.Cpu
	}{
		{
			name: "amd64",
			file: "cargueiro_darwin_amd64",
			cpu:  macho.CpuAmd64,
		},
		{
			name: "arm64",
			file: "cargueiro_darwin_arm64",
			cpu:  macho.CpuArm64,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := macho.Open(binary(t, tt.file))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, f.Close()) })

			require.Equal(t, tt.cpu, f.Cpu)
		})
	}
}

// TestLinuxBuildIsStamped runs the one build the host can run, and checks that
// it prints exactly the version it was built with: VERSION when make was given
// one, "dev" otherwise.
func TestLinuxBuildIsStamped(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("the linux/amd64 build can only run on linux/amd64")
	}

	out, err := exec.Command(binary(t, "cargueiro_linux_amd64"), "version").Output()
	require.NoError(t, err)

	want := os.Getenv("VERSION")
	if want == "" {
		want = "dev"
	}
	require.Equal(t, want, strings.TrimSpace(string(out)))
}
