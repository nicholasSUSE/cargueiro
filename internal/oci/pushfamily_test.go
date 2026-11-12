package oci

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/registry/remote/errcode"
)

// timeoutError is a network error that is a timeout.
type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func responseError(status int) error {
	return &errcode.ErrorResponse{Method: "PUT", URL: &url.URL{Scheme: "https", Host: "reg.local"}, StatusCode: status}
}

// flakyClient pushes and says what it stores, and fails the pushes it is told to.
type flakyClient struct {
	// pushErrs are the errors of the attempts to push a reference, in order. An
	// attempt after the last one succeeds.
	pushErrs map[string][]error
	// storedOnFailure lists the references that are stored even when the push
	// of them fails, as when the answer of the registry is lost.
	storedOnFailure []string
	// stored are the references the registry has.
	stored []string
	// existsErrs are the errors of the questions about a reference, in order.
	existsErrs map[string][]error

	pushes  []string
	checked []string
}

func (f *flakyClient) Push(_ []byte, ref string) (string, error) {
	f.pushes = append(f.pushes, ref)
	if errs := f.pushErrs[ref]; len(errs) > 0 {
		err := errs[0]
		f.pushErrs[ref] = errs[1:]
		if slices.Contains(f.storedOnFailure, ref) {
			f.stored = append(f.stored, ref)
		}
		return "", err
	}
	f.stored = append(f.stored, ref)
	return "stored/" + ref, nil
}

func (f *flakyClient) Exists(ref string) (bool, error) {
	f.checked = append(f.checked, ref)
	if errs := f.existsErrs[ref]; len(errs) > 0 {
		err := errs[0]
		f.existsErrs[ref] = errs[1:]
		return false, err
	}
	return slices.Contains(f.stored, ref), nil
}

func (f *flakyClient) ChartDigest(string) (string, error) {
	return "", errors.New("flakyClient has no digests")
}

// clock records the waits instead of waiting.
type clock struct {
	waits []time.Duration
	err   error
}

func (c *clock) sleep(_ context.Context, d time.Duration) error {
	c.waits = append(c.waits, d)
	return c.err
}

func retryWith(c *clock) Retry {
	r := DefaultRetry()
	r.Sleep = c.sleep
	return r
}

// familyGuard is the guard of the tests of the other charts, with the three
// charts of the fleet family in the edition of the Rancher charts.
func familyGuard(t *testing.T) *Guard {
	t.Helper()

	guard, err := NewGuard(Policy{
		Images: []string{"stgregistry.suse.com/rancher", "registry.suse.com/rancher"},
		Charts: map[string][]string{
			"rancher-charts": {"stgregistry.suse.com/rancher/catalog", "registry.suse.com/rancher/catalog"},
			"prime-charts":   {"stgregistry.suse.com/rancher/charts", "registry.suse.com/rancher/charts"},
		},
		Editions: map[string]string{
			"fleet":            "rancher-charts",
			"fleet-agent":      "rancher-charts",
			"fleet-crd":        "rancher-charts",
			"rancher-ai-agent": "prime-charts",
		},
	})
	require.NoError(t, err)
	return guard
}

// guardedTarget() is a registry the rules protect, and localTarget() one they do not.
func guardedTarget() Target { return Target{Host: "stgregistry.suse.com", Path: "rancher/catalog"} }

func localTarget() Target { return Target{Host: "reg.local:5000", Path: "team"} }

// familyArchives makes the archives of the fleet family, in the order of the family.
func familyArchives(t *testing.T) []string {
	t.Helper()

	return []string{
		writeChart(t, "fleet", "110.0.3+up0.16.3"),
		writeChart(t, "fleet-agent", "110.0.3+up0.16.3"),
		writeChart(t, "fleet-crd", "110.0.3+up0.16.3"),
	}
}

func addresses(target Target) []string {
	return []string{
		target.Ref("fleet", "110.0.3+up0.16.3"),
		target.Ref("fleet-agent", "110.0.3+up0.16.3"),
		target.Ref("fleet-crd", "110.0.3+up0.16.3"),
	}
}

func states(result *FamilyResult) []MemberState {
	var got []MemberState
	for _, m := range result.Members {
		got = append(got, m.State)
	}
	return got
}

func TestPushFamilyPushesEveryChartInOrder(t *testing.T) {
	fake := &flakyClient{}
	c := &clock{}

	result, err := NewPusher(fake, localTarget(), familyGuard(t)).WithRetry(retryWith(c)).
		PushFamily(context.Background(), familyArchives(t), nil)

	require.NoError(t, err)
	require.Equal(t, addresses(localTarget()), fake.pushes)
	require.Empty(t, fake.checked, "a registry that is not protected is not asked, as for a single chart")
	require.Equal(t, []MemberState{StatePushed, StatePushed, StatePushed}, states(result))
	require.Equal(t, "stored/"+addresses(localTarget())[0], result.Members[0].Ref)
}

func TestPushFamilySkipsWhatTheProtectedRegistryHasAndPushesTheRest(t *testing.T) {
	want := addresses(guardedTarget())
	fake := &flakyClient{stored: []string{want[0], want[1]}}

	result, err := NewPusher(fake, guardedTarget(), familyGuard(t)).WithRetry(retryWith(&clock{})).
		PushFamily(context.Background(), familyArchives(t), nil)

	require.NoError(t, err)
	require.Equal(t, want, fake.checked, "one question for each chart, before any upload")
	require.Equal(t, []string{want[2]}, fake.pushes, "only the chart that is not there is uploaded")
	require.Equal(t, []MemberState{StateSkipped, StateSkipped, StatePushed}, states(result))
	require.Equal(t, "stgregistry.suse.com/rancher/catalog/fleet:110.0.3_up0.16.3", result.Members[0].Ref,
		"the reference of a chart that was there is written as the registry stores it")
}

func TestPushFamilyUploadsNothingWhenTheGuardRefusesOne(t *testing.T) {
	fake := &flakyClient{}
	archives := []string{
		writeChart(t, "fleet", "110.0.3+up0.16.3"),
		// A chart of the other edition cannot go to this path.
		writeChart(t, "rancher-ai-agent", "110.0.3+up0.16.3"),
	}

	result, err := NewPusher(fake, guardedTarget(), familyGuard(t)).PushFamily(context.Background(), archives, nil)

	require.ErrorContains(t, err, "rancher-ai-agent")
	require.Nil(t, result)
	require.Empty(t, fake.checked, "the network is not touched")
	require.Empty(t, fake.pushes)
}

func TestPushFamilyTriesAgainWhenAnUploadCanPass(t *testing.T) {
	want := addresses(localTarget())
	fake := &flakyClient{pushErrs: map[string][]error{want[1]: {responseError(503), timeoutError{}}}}
	c := &clock{}
	var notices []int

	result, err := NewPusher(fake, localTarget(), familyGuard(t)).WithRetry(retryWith(c)).
		PushFamily(context.Background(), familyArchives(t), func(chart string, attempt, attempts int, _ error, _ time.Duration) {
			require.Equal(t, "fleet-agent", chart)
			require.Equal(t, 5, attempts)
			notices = append(notices, attempt)
		})

	require.NoError(t, err)
	require.Equal(t, []time.Duration{30 * time.Second, 60 * time.Second}, c.waits, "the wait doubles")
	require.Equal(t, []int{1, 2}, notices)
	require.Equal(t, []MemberState{StatePushed, StatePushed, StatePushed}, states(result))
	require.Equal(t, 5, len(fake.pushes), "the chart that failed twice was uploaded three times")
}

func TestPushFamilyGivesUpAfterTheLastAttemptAndLeavesTheRestUntried(t *testing.T) {
	want := addresses(localTarget())
	fake := &flakyClient{pushErrs: map[string][]error{
		want[1]: {timeoutError{}, timeoutError{}, timeoutError{}, timeoutError{}, timeoutError{}, timeoutError{}},
	}}
	c := &clock{}

	result, err := NewPusher(fake, localTarget(), familyGuard(t)).WithRetry(retryWith(c)).
		PushFamily(context.Background(), familyArchives(t), nil)

	require.ErrorContains(t, err, "i/o timeout")
	require.Equal(t, []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second, 240 * time.Second}, c.waits)
	require.Equal(t, []MemberState{StatePushed, StateFailed, StateNotTried}, states(result))
	require.Error(t, result.Members[1].Err)
	require.Empty(t, result.Members[2].Ref)
	require.Len(t, fake.pushes, 1+5, "the first chart once, the second five times, the third never")
}

func TestPushFamilyDoesNotTryAgainWhatCannotPass(t *testing.T) {
	for _, status := range []int{400, 401, 403, 404} {
		t.Run(responseError(status).Error(), func(t *testing.T) {
			want := addresses(localTarget())
			fake := &flakyClient{pushErrs: map[string][]error{want[0]: {responseError(status)}}}
			c := &clock{}

			result, err := NewPusher(fake, localTarget(), familyGuard(t)).WithRetry(retryWith(c)).
				PushFamily(context.Background(), familyArchives(t), nil)

			require.Error(t, err)
			require.Empty(t, c.waits)
			require.Len(t, fake.pushes, 1)
			require.Equal(t, []MemberState{StateFailed, StateNotTried, StateNotTried}, states(result))
		})
	}
}

func TestPushFamilyCountsAChartAsPushedWhenTheAnswerWasLost(t *testing.T) {
	want := addresses(guardedTarget())
	fake := &flakyClient{
		pushErrs:        map[string][]error{want[1]: {timeoutError{}}},
		storedOnFailure: []string{want[1]},
	}
	c := &clock{}

	result, err := NewPusher(fake, guardedTarget(), familyGuard(t)).WithRetry(retryWith(c)).
		PushFamily(context.Background(), familyArchives(t), nil)

	require.NoError(t, err)
	require.Empty(t, c.waits, "it did not wait to try again: the chart was there")
	require.Equal(t, []MemberState{StatePushed, StatePushed, StatePushed}, states(result))
	require.Equal(t, 3, len(fake.pushes), "and it was not uploaded twice")
}

func TestPushFamilyTriesTheQuestionAgainBeforeUploadingAnything(t *testing.T) {
	want := addresses(guardedTarget())
	fake := &flakyClient{existsErrs: map[string][]error{want[0]: {timeoutError{}, timeoutError{}, timeoutError{}, timeoutError{}, timeoutError{}}}}
	c := &clock{}

	result, err := NewPusher(fake, guardedTarget(), familyGuard(t)).WithRetry(retryWith(c)).
		PushFamily(context.Background(), familyArchives(t), nil)

	require.ErrorContains(t, err, "check whether "+want[0]+" exists")
	require.Nil(t, result)
	require.Len(t, c.waits, 4)
	require.Empty(t, fake.pushes, "nothing is uploaded when the state of the registry is not known")
}

func TestPushFamilyStopsWaitingWhenTheContextIsDone(t *testing.T) {
	want := addresses(localTarget())
	fake := &flakyClient{pushErrs: map[string][]error{want[0]: {timeoutError{}, timeoutError{}}}}
	c := &clock{err: context.Canceled}

	result, err := NewPusher(fake, localTarget(), familyGuard(t)).WithRetry(retryWith(c)).
		PushFamily(context.Background(), familyArchives(t), nil)

	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, c.waits, 1)
	require.Equal(t, []MemberState{StateFailed, StateNotTried, StateNotTried}, states(result))
}

func TestRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "too many requests", err: responseError(429), want: true},
		{name: "server error", err: responseError(500), want: true},
		{name: "bad gateway", err: responseError(502), want: true},
		{name: "unavailable", err: responseError(503), want: true},
		{name: "bad request", err: responseError(400), want: false},
		{name: "unauthorized", err: responseError(401), want: false},
		{name: "forbidden", err: responseError(403), want: false},
		{name: "not found", err: responseError(404), want: false},
		{name: "a timeout", err: timeoutError{}, want: true},
		{name: "a connection that was refused", err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}, want: true},
		{name: "a wrapped network error", err: errors.Join(errors.New("push"), &url.Error{Op: "Put", Err: timeoutError{}}), want: true},
		{name: "a connection that was cut", err: io.ErrUnexpectedEOF, want: true},
		{name: "end of file", err: io.EOF, want: true},
		{name: "a certificate that is not trusted", err: &url.Error{Op: "Get", Err: &tls.CertificateVerificationError{}}, want: false},
		{name: "a host that does not exist", err: &net.DNSError{Err: "no such host", IsNotFound: true}, want: false},
		{name: "any other error", err: errors.New("read archive: no such file"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, retryable(tt.err))
		})
	}
}

func TestRetryWithoutAttemptsRunsOnce(t *testing.T) {
	calls := 0

	err := Retry{}.do(context.Background(), func() error { calls++; return timeoutError{} }, nil)

	require.Error(t, err)
	require.Equal(t, 1, calls)
}
