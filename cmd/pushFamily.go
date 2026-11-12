package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/rancher/cargueiro/internal/family"
	"github.com/rancher/cargueiro/internal/oci"
	"github.com/rancher/cargueiro/internal/rules"
)

// archiveIdentity is a chart archive, and the name and version in it.
type archiveIdentity struct {
	path    string
	name    string
	version string
}

// pushFamily pushes the family called name to registry: archives must be exactly
// the charts of the family, in any order, all of the same version. They are
// pushed in the order of the family, and a failure stops it and says what
// happened to each chart, so that running the command again finishes the rest.
func pushFamily(cmd *cobra.Command, opts *rootOptions, newPusher pusherFactory, name string, archives []string, registry string, creds *credentialFlags) error {
	target, err := oci.ParseTarget(registry)
	if err != nil {
		return err
	}
	guard, err := newGuard(opts.rules)
	if err != nil {
		return err
	}
	cfg, err := rules.Parse(opts.rules)
	if err != nil {
		return err
	}
	members, err := cfg.Family(name)
	if err != nil {
		return err
	}
	ordered, err := orderArchives(members, archives)
	if err != nil {
		return err
	}

	// Before reading any secret or touching the network.
	if guard.Guarded(target) {
		var problems []error
		for _, a := range ordered {
			if err := guard.Check(target, a.name); err != nil {
				problems = append(problems, err)
			}
		}
		if err := errors.Join(problems...); err != nil {
			return err
		}
	}

	given := creds.read(opts)
	conn, err := given.connection(cmd.Context(), target.Host)
	if err != nil {
		return err
	}
	pusher, err := newPusher(target, guard, conn.clientOptions())
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(ordered))
	for _, a := range ordered {
		paths = append(paths, a.path)
	}
	opts.log.DebugContext(cmd.Context(), "pushing family", "family", name, "registry", registry)

	notice := func(chart string, attempt, attempts int, err error, wait time.Duration) {
		warnf(opts.stderr, "%s: attempt %d of %d failed (%v): trying again in %s", chart, attempt, attempts, err, wait)
	}
	result, err := pusher.PushFamily(cmd.Context(), paths, notice)
	if err != nil {
		if result == nil {
			return fmt.Errorf("nothing was pushed: %w", err)
		}
		warnf(opts.stderr, "the family %q was not pushed completely:\n%s", name, familyReport(result))
		return fmt.Errorf("%w: run the command again to finish it", err)
	}

	for _, m := range result.Members {
		if m.State == oci.StateSkipped {
			warnf(opts.stderr, "%s %s is in the registry already, so it was not pushed", m.Chart, m.Version)
		}
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "oci://"+m.Ref); err != nil {
			return fmt.Errorf("write reference: %w", err)
		}
	}
	return nil
}

// orderArchives reads the name and the version of every archive, checks that the
// archives are exactly the charts of the family and of one version, and returns
// them in the order of the family.
func orderArchives(members rules.Family, archives []string) ([]archiveIdentity, error) {
	identities := make([]archiveIdentity, 0, len(archives))
	var problems []error
	for _, path := range archives {
		name, version, err := oci.ReadIdentity(path)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		identities = append(identities, archiveIdentity{path: path, name: name, version: version})
	}
	if err := errors.Join(problems...); err != nil {
		return nil, err
	}

	ordered, err := family.Resolve(members, identities, func(a archiveIdentity) (string, error) { return a.name, nil })
	if err != nil {
		return nil, err
	}

	var same = true
	versions := make([]string, 0, len(ordered))
	for _, a := range ordered {
		versions = append(versions, a.name+" "+a.version)
		same = same && a.version == ordered[0].version
	}
	if !same {
		return nil, fmt.Errorf("the charts of the family %q do not have the same version: %s", members.Name, strings.Join(versions, ", "))
	}
	return ordered, nil
}

// familyReport says what happened to each chart: the ones pushed, the ones that
// were there, the one that failed and why, and the ones not tried.
func familyReport(result *oci.FamilyResult) string {
	var lines []string
	for _, state := range []oci.MemberState{oci.StatePushed, oci.StateSkipped, oci.StateFailed, oci.StateNotTried} {
		var charts []string
		for _, m := range result.Members {
			if m.State != state {
				continue
			}
			chart := m.Chart + " " + m.Version
			if m.Err != nil {
				chart += " (" + m.Err.Error() + ")"
			}
			charts = append(charts, chart)
		}
		list := "-"
		if len(charts) > 0 {
			list = strings.Join(charts, ", ")
		}
		lines = append(lines, fmt.Sprintf("  %-14s %s", string(state)+":", list))
	}
	return strings.Join(lines, "\n")
}
