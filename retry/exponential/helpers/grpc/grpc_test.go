package grpc

import (
	"fmt"
	"testing"

	"github.com/gostdlib/foundation/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrTransformer(t *testing.T) {
	t.Parallel()

	tr := New(codes.NotFound)
	for i := 1; i < 16; i++ { // 16 is the max Code in gRPC at this time and 0 is OK
		wantPermErr := true
		code := codes.Code(i)
		if grpcRetriable[code] || code == codes.NotFound {
			wantPermErr = false
		}
		err := status.Error(code, "test error")
		got := tr.ErrTransformer(err)

		permErr := errors.Is(got, errors.ErrPermanent)
		if permErr != wantPermErr {
			t.Errorf("TestErrTransformer(%s): wrong error type for code", code)
		}
	}
}

func TestIsGRPCErr(t *testing.T) {
	t.Parallel()

	tr := &Transformer{}

	tests := []struct {
		name string
		err  error
		code codes.Code
		want bool
	}{
		{
			name: "Non-gRPC error",
			err:  fmt.Errorf("not a grpc error"),
			code: codes.Unknown,
			want: false,
		},
		{
			name: "gRPC error with codes.Unknown",
			err:  status.Error(codes.Unknown, "unknown error"),
			code: codes.Unknown,
			want: true,
		},
		{
			name: "gRPC error",
			err:  status.Error(codes.Unavailable, "transient error"),
			code: codes.Unavailable,
			want: true,
		},
		{
			name: "gRPC error, but status code is OK",
			err:  status.Error(codes.OK, "OK"),
			code: codes.OK,
			want: false,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			gotOk, code := tr.isGRPCErr(test.err)
			if gotOk != test.want {
				t.Errorf("isGRPCErr(): got %v, want %v", gotOk, test.want)
			}
			if code != test.code {
				t.Errorf("isGRPCErr(): got %v, want %v", code, test.code)
			}
		})
	}
}

func TestIsGRPCPermanent(t *testing.T) {
	t.Parallel()

	tr := &Transformer{}

	for code := range grpcRetriable {
		code := code
		t.Run(code.String(), func(t *testing.T) {
			if got := tr.isGRPCPermanent(code); got {
				t.Errorf("isGRPCPermanent(): got %v, want %v", got, false)
			}
		})
	}
	if got := tr.isGRPCPermanent(codes.PermissionDenied); !got {
		t.Errorf("isGRPCPermanent(%v): got %v, want %v", codes.PermissionDenied, got, true)
	}
}
