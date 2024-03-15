/*
Package grpc provides an exponential.ErrTransformer that can be used to detect non-retriable errors for gRPC calls.

Example:
	grpcErrTransform := grpc.New() // Uses defaults

	backoff := exponential.WithErrTransformer(grpcErrTransform)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	req := &pb.HelloRequest{Name: "John"}
	var resp *pb.HelloReply{}

	err := backoff.Retry(
		ctx,
		func(ctx context.Context, r Record) error {
			var err error
			resp, err = client.SayHello(ctx, req)
			return err
		},
	)
	cancel()
*/
package grpc

import (
	"fmt"
	"reflect"

	"github.com/gostdlib/foundation/errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/*
Transformer provides an ErrTransformer method that can be used to detect non-retriable errors.
The following codes are retriable: Canceled, DeadlineExceeded, Unknown, Internal, Unavailable, ResourceExhausted.
Any other code is not.
*/
type Transformer struct {
	extras map[codes.Code]bool
}

// New returns a new Transformer. This implements exponential.ErrTransformer with the method ErrTransformer.
// You can add other codes that are retriable by passing them as arguments. This list of retriable codes
// are listed on Transformer.
func New(extras ...codes.Code) *Transformer {
	m := make(map[codes.Code]bool, len(extras))
	for _, code := range extras {
		m[code] = true
	}
	return &Transformer{extras: m}
}

// ErrTransformer returns a transformer that can be used to detect non-retriable errors.
// If it is non-retriable it will wrap the error with errors.ErrPermanent.
func(t *Transformer) ErrTransformer(err error) error {
	is, code := t.isGRPCErr(err)
	if !is {
		return err
	}

	if t.isGRPCPermanent(code) {
		return fmt.Errorf("%w: %w", err, errors.ErrPermanent)
	}
	return err
}

// isGRPCErr returns true if the error is a gRPC error and the gRPC code.
func (t *Transformer) isGRPCErr(err error) (bool, codes.Code) {
	// The gRPC status package is actually a wrapper around an internal status package. While Status is exposed
	// through this package, the Error type is not. So there is no great way to know if
	// we have a grpc Error type. That is unless we want to use the compiler linkname directive to get
	// at the internal status package. So instead we look to see if codes.Unknown is returned, which
	// is what happens when we have a non-gRPC error given to code. But since a person can set that too,
	// we look to see if the error has a GRPCStatus method. If it does, then it is a gRPC error.
	// The tests should protect us in case they change the internal Error type to remove GRPCStatus.
	code := status.Code(err)
	switch code {
	case codes.Unknown:
		// We look to see if the error has a GRPCStatus method. If it does, then it is a gRPC error.
		// This is not the greatest, but it is the best we can do without using the compiler directive.
		if _, ok := reflect.TypeOf(err).MethodByName("GRPCStatus"); ok {
			return true, code
		}
		return false, code
	case codes.OK:
		return false, code
	}
	return true, code
}

// grpcRetriable is a list of grpc status codes that are retriable.
var grpcRetriable = map[codes.Code]bool{
	codes.Canceled:          true,
	codes.DeadlineExceeded:  true,
	codes.Unknown:           true,
	codes.Internal:          true,
	codes.Unavailable:       true,
	codes.ResourceExhausted: true,
}

// isGRPCPermanent returns true if the error is a GRPC error that is permanent.
func (t *Transformer) isGRPCPermanent(code codes.Code) bool {
	if grpcRetriable[code] {
		return false
	}
	if t.extras[code] {
		return false
	}
	return true
}
