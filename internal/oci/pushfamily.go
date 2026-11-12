package oci

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"

	"github.com/rancher/cargueiro/internal/version"
)

// MemberState says what happened to a chart of a family.
type MemberState string

// The states of a chart of a family.
const (
	// StatePushed: the chart was uploaded by this run.
	StatePushed MemberState = "pushed"
	// StateSkipped: the registry had the chart already.
	StateSkipped MemberState = "already there"
	// StateFailed: the upload failed, after every attempt.
	StateFailed MemberState = "failed"
	// StateNotTried: an earlier chart failed, so this one was not uploaded.
	StateNotTried MemberState = "not tried"
)

// MemberResult is what happened to one chart of a family.
type MemberResult struct {
	Chart   string
	Version string
	State   MemberState
	// Ref is the reference the chart is stored under. It is empty when the
	// upload failed or was not tried.
	Ref string
	// Err is why the upload failed.
	Err error
}

// FamilyResult is what happened to every chart of a family, in the order the
// archives were given.
type FamilyResult struct {
	Members []MemberResult
}

// WithRetry returns a Pusher that runs the uploads of a family again, as r says,
// when they fail in a way that can pass.
func (p *Pusher) WithRetry(r Retry) *Pusher {
	pusher := *p
	pusher.retry = r
	return &pusher
}

// RetryNotice is told that an upload of a chart failed and will be tried again.
type RetryNotice func(chart string, attempt, attempts int, err error, wait time.Duration)

// PushFamily uploads the archives, one after the other, in the order given, which
// is the order of the family. Nothing is uploaded unless every archive is a chart
// the guard allows. On a registry the guard protects, a chart that the registry
// has already is skipped, and never overwritten, but only when what is stored is
// the archive given: if its digest is other, nothing is uploaded and the error
// says which charts differ. An upload that fails in a way
// that can pass is tried again, as the Pusher's Retry says.
//
// When an upload fails for good, the charts after it are not tried. The result
// is returned with the error, and says what happened to every chart.
func (p *Pusher) PushFamily(ctx context.Context, archives []string, notice RetryNotice) (*FamilyResult, error) {
	type archive struct {
		data    []byte
		address string
		sum     string
	}
	pending := make([]archive, 0, len(archives))
	result := &FamilyResult{Members: make([]MemberResult, 0, len(archives))}
	var problems []error
	for _, path := range archives {
		data, err := os.ReadFile(path)
		if err != nil {
			problems = append(problems, fmt.Errorf("read archive %s: %w", path, err))
			continue
		}
		name, ver, err := chartIdentity(data)
		if err != nil {
			problems = append(problems, fmt.Errorf("invalid chart archive: %s: %w", path, err))
			continue
		}
		if err := p.guard.Check(p.target, name); err != nil {
			problems = append(problems, err)
			continue
		}
		pending = append(pending, archive{data: data, address: p.target.Ref(name, ver), sum: digest.FromBytes(data).String()})
		result.Members = append(result.Members, MemberResult{Chart: name, Version: ver, State: StateNotTried})
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}

	guarded := p.guard.Guarded(p.target)
	if guarded {
		var differ []error
		for i, a := range pending {
			exists, err := p.existsWithRetry(ctx, a.address)
			if err != nil {
				return nil, fmt.Errorf("check whether %s exists: %w", a.address, err)
			}
			if !exists {
				continue
			}
			stored, err := p.chartDigestWithRetry(ctx, a.address)
			if err != nil {
				return nil, fmt.Errorf("read the digest of %s: %w", a.address, err)
			}
			if stored != a.sum {
				differ = append(differ, refused("%s is in the registry already with other content (digest %s, the archive has %s): a version is never overwritten, so give the chart a new version", a.address, stored, a.sum))
				continue
			}
			result.Members[i].State = StateSkipped
			result.Members[i].Ref = withOCITag(a.address)
		}
		if len(differ) > 0 {
			return nil, errors.Join(differ...)
		}
	}

	for i, a := range pending {
		member := &result.Members[i]
		if member.State == StateSkipped {
			continue
		}
		ref, err := p.pushWithRetry(ctx, a.data, a.address, guarded, func(attempt int, err error, wait time.Duration) {
			if notice != nil {
				notice(member.Chart, attempt, max(p.retry.Attempts, 1), err, wait)
			}
		})
		if err != nil {
			member.State = StateFailed
			member.Err = err
			return result, fmt.Errorf("push %s: %w", a.address, err)
		}
		member.State, member.Ref = StatePushed, ref
	}
	return result, nil
}

// existsWithRetry asks whether ref is stored, and asks again when the question
// fails in a way that can pass.
func (p *Pusher) existsWithRetry(ctx context.Context, ref string) (bool, error) {
	var exists bool
	err := p.retry.do(ctx, func() error {
		var err error
		exists, err = p.client.Exists(ref)
		return err
	}, nil)
	return exists, err
}

// pushWithRetry uploads data to address. When an attempt fails on a registry the
// guard protects, and the registry has the chart afterwards, the answer of the
// upload was lost, and the chart counts as pushed.
func (p *Pusher) pushWithRetry(ctx context.Context, data []byte, address string, guarded bool, onRetry func(int, error, time.Duration)) (string, error) {
	var ref string
	retry := p.retry
	retry.OnRetry = onRetry
	err := retry.do(ctx, func() error {
		var err error
		ref, err = p.client.Push(data, address)
		return err
	}, func() bool {
		if !guarded {
			return false
		}
		exists, err := p.client.Exists(address)
		return err == nil && exists
	})
	if err != nil {
		return "", err
	}
	if ref == "" {
		ref = withOCITag(address)
	}
	return ref, nil
}

// withOCITag writes the version at the end of ref as the registry stores it.
func withOCITag(ref string) string {
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		return ref[:i+1] + version.ToOCITag(ref[i+1:])
	}
	return ref
}

// chartDigestWithRetry reads the digest of the chart stored at ref, and reads it
// again when the read fails in a way that can pass.
func (p *Pusher) chartDigestWithRetry(ctx context.Context, ref string) (string, error) {
	var sum string
	err := p.retry.do(ctx, func() error {
		var err error
		sum, err = p.client.ChartDigest(ref)
		return err
	}, nil)
	return sum, err
}
