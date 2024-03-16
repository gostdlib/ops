package http

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/gostdlib/ops/retry/internal/errors"
	"github.com/kylelemons/godebug/pretty"
)

type tmpErr struct{}

func (t tmpErr) Error() string {
	return "temporary error"

}

func (t tmpErr) Temporary() bool {
	return true
}

func TestErrTransformer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		errArg  error
		wantErr error
	}{
		{
			name:    "nil error",
			errArg:  nil,
			wantErr: nil,
		},
		{
			name:    "non-http error",
			errArg:  fmt.Errorf("some error"),
			wantErr: fmt.Errorf("some error"),
		},
		{
			name:    "http temporary error",
			errArg:  &url.Error{Err: tmpErr{}},
			wantErr: &url.Error{Err: tmpErr{}},
		},
		{
			name:    "http non-temporary error",
			errArg:  &url.Error{Err: fmt.Errorf("some error")},
			wantErr: fmt.Errorf("%w: %w", &url.Error{Err: fmt.Errorf("some error")}, errors.ErrPermanent),
		},
	}

	for _, test := range tests {
		tr := &Transformer{}
		got := tr.ErrTransformer(test.errArg)
		if diff := pretty.Compare(test.wantErr, got); diff != "" {
			t.Errorf("TestErrTransformer(%s): -want/+got:\n%s", test.name, diff)
		}
	}

}

func TestRespToErr(t *testing.T) {
	t.Parallel()

	someErr := fmt.Errorf("some error")
	respHadErr := fmt.Errorf("response error")
	unexpectedErr := fmt.Errorf("unexpected response")

	tests := []struct {
		name       string
		respArg    *http.Response
		errArg     error
		respToErrs []RespToErr
		wantErr    error
	}{
		{
			name:    "error already set",
			respArg: &http.Response{},
			errArg:  someErr,
			respToErrs: []RespToErr{
				func(r *http.Response) error {
					return unexpectedErr
				},
			},
			wantErr: someErr,
		},
		{
			name:    "error was nil and no transformers",
			respArg: &http.Response{},
			errArg:  nil,
			wantErr: nil,
		},
		{
			name:    "error was not nil and transformer returns nil",
			respArg: &http.Response{},
			errArg:  someErr,
			respToErrs: []RespToErr{
				func(r *http.Response) error {
					return nil
				},
			},
			wantErr: someErr,
		},
		{
			name:    "error was not nil and transformer returns error, but we shouldn't get to it",
			respArg: &http.Response{},
			errArg:  someErr,
			respToErrs: []RespToErr{
				func(r *http.Response) error {
					return respHadErr
				},
			},
			wantErr: someErr,
		},
		{
			name:    "error was nil, two transformers fire, should get wrapped error",
			respArg: &http.Response{},
			errArg:  nil,
			respToErrs: []RespToErr{
				func(r *http.Response) error {
					return respHadErr
				},
				func(r *http.Response) error {
					return unexpectedErr
				},
			},
			wantErr: fmt.Errorf("%w: %w", respHadErr, unexpectedErr),
		},
		{
			name:    "error was nil, two transformers but first is ErrPermanent",
			respArg: &http.Response{},
			errArg:  nil,
			respToErrs: []RespToErr{
				func(r *http.Response) error {
					return fmt.Errorf("%w: %w", someErr, errors.ErrPermanent)
				},
				func(r *http.Response) error {
					return unexpectedErr
				},
			},
			wantErr: fmt.Errorf("%w: %w", someErr, errors.ErrPermanent),
		},
	}

	for _, test := range tests {
		tr := &Transformer{respToErrs: test.respToErrs}
		gotResp, gotErr := tr.RespToErr(test.respArg, test.errArg)
		if diff := pretty.Compare(test.respArg, gotResp); diff != "" {
			t.Errorf("TestRespToErr(%s): -want/+got:\n%s", test.name, diff)
			continue
		}
		if diff := pretty.Compare(test.wantErr, gotErr); diff != "" {
			t.Errorf("TestRespToErr(%s): -want/+got:\n%s", test.name, diff)
		}
	}
}
