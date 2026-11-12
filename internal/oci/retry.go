package oci

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"time"

	"oras.land/oras-go/v2/registry/remote/errcode"
)

// Retry runs an operation again when it fails in a way that can pass, such as a
// network error. The zero value runs it once.
type Retry struct {
	// Attempts is the most times the operation runs.
	Attempts int
	// Wait is the time to wait after the first failure. It doubles after each
	// one.
	Wait time.Duration
	// Sleep waits for d, or until ctx is done. It is time based when nil.
	Sleep func(ctx context.Context, d time.Duration) error
	// OnRetry is called before each wait, with the attempt that failed.
	OnRetry func(attempt int, err error, wait time.Duration)
}

// DefaultRetry is 5 attempts, waiting 30 seconds after the first failure and
// twice as long after each one that follows: 30, 60, 120 and 240 seconds.
func DefaultRetry() Retry {
	return Retry{Attempts: 5, Wait: 30 * time.Second}
}

// do runs op until it succeeds, fails with an error that cannot pass, or the
// attempts end. After each failure, done says whether the work is done in spite
// of it, such as a push whose answer was lost, and then do succeeds. done may be
// nil.
func (r Retry) do(ctx context.Context, op func() error, done func() bool) error {
	attempts := max(r.Attempts, 1)
	wait := r.Wait
	var err error
	for attempt := 1; ; attempt++ {
		if err = op(); err == nil {
			return nil
		}
		if done != nil && done() {
			return nil
		}
		if attempt >= attempts || !retryable(err) {
			return err
		}
		if r.OnRetry != nil {
			r.OnRetry(attempt, err, wait)
		}
		if sleepErr := r.sleep(ctx, wait); sleepErr != nil {
			return errors.Join(err, sleepErr)
		}
		wait *= 2
	}
}

func (r Retry) sleep(ctx context.Context, d time.Duration) error {
	if r.Sleep != nil {
		return r.Sleep(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// retryable reports whether an error is one that can pass: a network error or a
// timeout, a connection that was cut, and an answer of the registry that says it
// is busy (429) or broken (5xx). An answer that refuses the request (400, 401,
// 403, 404), a certificate that is not trusted, and a host that does not exist
// are not.
func retryable(err error) bool {
	var resp *errcode.ErrorResponse
	var cert *tls.CertificateVerificationError
	var dns *net.DNSError
	var network net.Error
	switch {
	case errors.As(err, &resp):
		return resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= http.StatusInternalServerError
	case errors.As(err, &cert):
		return false
	case errors.As(err, &dns) && dns.IsNotFound:
		return false
	case errors.As(err, &network):
		return true
	}
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
