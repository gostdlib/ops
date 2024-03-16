/*
Package http provides an ErrTransformer for http.Client from the standard library.
Other third-party HTTP clients are not supported by this package.

Example that handle HTTP non-temporary error codes:

		httpTransform := http.New()

		backoff := exponential.WithErrTransformer(httpTransform)
	    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	    var resp *http.Response

	    err := backoff.Retry(
	    	ctx,
	     	func(ctx context.Context, r Record) error {
	      		var err error
	        	resp, err = httpClient.Do(someRequest)
	         	return err
	        },
	    )
	    cancel()

Example with custom errors:

		bodyHasErr := func(r *http.Response) error {
	 		b, err :io.ReadAll(r.Body)
	 		if err != nil {
	 			return fmt.Errorf("response body had error: %s", err)
	    	}

			s := strings.TrimSpace(string(b))
	 		if strings.HasPrefix(s, "error") {
	 			if strings.Contains(s, "errors: permament") {
	 				return fmt.Errorf("error: %w: %w", s, errors.ErrPermanent)
	 			}
	 			return fmt.Errorf("error: %s", s)
			}
	 		return nil
	   }

	   httpTransform := http.New(bodyHasErr)

	   backoff := exponential.WithErrTransformer(httpTransform)
	   ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

	   var resp *http.Response

	   err := backoff.Retry(
	   		ctx,
	     	func(ctx context.Context, r Record) error {
	      		var err error
	        	resp, err = httpTransform.RespToErr(httpClient.Do(someRequest)) // <- note the call wrapper
	         	return err
	        },
	    )
	    cancel()
*/
package http

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/gostdlib/ops/retry/internal/errors"
)

// Transformer provides an ErrTransformer method that can be used to detect non-retriable errors.
// The following codes are retriable: StatusRequestTimeout, StatusConflict, StatusLocked, StatusTooEarly,
// StatusTooManyRequests, StatusInternalServerError and StatusGatewayTimeout.
// Any other code is not.
type Transformer struct {
	respToErrs []RespToErr
}

// RespToErr allows you to inspect a Response and determine if the result is really an error.
// If you want to make that type of error non-retriable, wrap the error with errors.ErrPermanent, like
// so: return fmt.Errorf("had some error condition: %w", errors.ErrPermanent) . This should return
// nil if the Response was fine.
type RespToErr func(r *http.Response) error

// New returns a new Transformer. This implements exponential.ErrTransformer with the method ErrTransformer.
func New(respToErrs ...RespToErr) *Transformer {
	return &Transformer{respToErrs: respToErrs}
}

// ErrTransformer returns a transformer that can be used to detect non-retriable errors.
// If the error is of type *url.Error (the type returned by http.Client) and is .Temporary() == false,
// this will mark the error as a permanent error. Otherwise it will return the error.
// If it is non-retriable it will wrap the error with errors.ErrPermanent. This is meant to be used
// with .RespToErr() which will return an error based on the content of a http.Response.
func (t *Transformer) ErrTransformer(err error) error {
	switch e := err.(type) {
	case *url.Error:
		if !e.Temporary() {
			return fmt.Errorf("%w: %w", err, errors.ErrPermanent)
		}
		return err
	}
	return err
}

// RespToErr takes an http.Resp and an error from an http.Client call method and returns the Response
// and an error. If error != nil , this simply return the values passed. Otherwise it will inspect the
// Response accord to rules passed to New() to determine if we have an error. It will always execute
// all error RespToErr(s) unless the error returned is wrapped with ErrPermanent.
func (t *Transformer) RespToErr(r *http.Response, err error) (*http.Response, error) {
	if len(t.respToErrs) == 0 {
		return r, err
	}
	if err != nil {
		return r, err
	}

	var retErr error
	for _, respToErr := range t.respToErrs {
		wasPermanent := false
		if err = respToErr(r); err != nil {
			wasPermanent = errors.Is(err, errors.ErrPermanent)
			if retErr == nil {
				retErr = err
			} else {
				retErr = fmt.Errorf("%w: %w", retErr, err)
			}
			if wasPermanent {
				break
			}
		}
	}
	return r, retErr
}
