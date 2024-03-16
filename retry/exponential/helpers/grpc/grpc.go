/*
Package gRPC provides an exponential.ErrTransformer that can be used to detect non-retriable errors for gRPC calls.
There is no direct support for gRPC streaming in this package.

Example using just defaults:

	// This will retry any grpc error codes that are considered retriable.
	grpcErrTransform, _ := grpc.New() // Uses defaults

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

Example setting an extra code for retries:

	// The same as above, except we will retry on codes.DataLoss.
	grpcErrTransform, err := grpc.New(WithExtraCodes(codes.DataLoss))
	if err != nil {
		// Handle error
	}
	... // The rest is the same

Example with custom message inspection:

	// We are going to provide a function that can inspect a proto.Message when
	// the client did not send an error, but there was an error sent back from the server
	// in the response.
	respHasErr := func (msg proto.Message) error {
		r := msg.(*pb.HelloReply)

		if r.Error != "" {
			if r.PermanentErr {
				// This will stop retries.
				return fmt.Errorf("%s: %w", r.Error, errors.ErrPermanent)
			}
			// We can still retry.
			return fmt.Errorf("%s", r.Error)
		}
		return nil
	}
	grpcErrTransform, err := grpc.New(WithProtoToErr(respHasErr))
	if err != nil {
		// Handle error
	}

	backoff := exponential.WithErrTransformer(grpcErrTransform)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	req := &pb.HelloRequest{Name: "John"}
	var resp *pb.HelloReply{}

	err := backoff.Retry(
		ctx,
		func(ctx context.Context, r Record) error {
			a, err := grpcErrTransform.RespToErr(client.SayHello(ctx, req)) // <- Notice the call wrapper
			if err != nil {
				return err
			}
			resp = a.(*pb.HelloReply)
			return nil
		},
	)
	cancel()
*/
package grpc

import (
	"fmt"
	"reflect"

	"github.com/gostdlib/ops/retry/internal/errors"
	"google.golang.org/protobuf/proto"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/*
Transformer provides an ErrTransformer method that can be used to detect non-retriable errors.
The following codes are retriable: Canceled, DeadlineExceeded, Unknown, Internal, Unavailable, ResourceExhausted.
Any other code is not.
*/
type Transformer struct {
	extras       map[codes.Code]bool
	protosToErrs []ProtoToErr
}

// Option is an option for the New() constructor.
type Option func(t *Transformer) error

// WithExtraCodes defines extra grpc status codes that are considered retriable.
func WithExtraCodes(extras ...codes.Code) Option {
	return func(t *Transformer) error {
		for _, code := range extras {
			t.extras[code] = true
		}
		return nil
	}
}

// ProtoToErr inspects a protocol buffer message and determines if the call was really an error.
// If it was not, this returns nil.
type ProtoToErr func(msg proto.Message) error

// WithProtoToErrs pass functions that look at protocol buffer message responses to determine if
// the message actually indicates an error.
func WithProtoToErrs(protosToErrs ...ProtoToErr) Option {
	return func(t *Transformer) error {
		t.protosToErrs = protosToErrs
		return nil
	}
}

// New returns a new Transformer. This implements exponential.ErrTransformer with the method ErrTransformer.
// You can add other codes that are retriable by passing them as arguments. This list of retriable codes
// are listed on Transformer.
func New(options ...Option) (*Transformer, error) {
	t := &Transformer{
		extras: map[codes.Code]bool{},
	}

	for _, o := range options {
		if err := o(t); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// ErrTransformer returns a transformer that can be used to detect non-retriable errors.
// If it is non-retriable it will wrap the error with errors.ErrPermanent.
func (t *Transformer) ErrTransformer(err error) error {
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

// RespToErr takes a proto.Message and an error from a call from a protocol buffer client call method and
// returns the Response and an error. If error != nil , this simply return the values passed. Otherwise it will inspect the
// Response accord to rules passed to New() to determine if we have an error.
func (t *Transformer) RespToErr(r proto.Message, err error) (proto.Message, error) {
	if len(t.protosToErrs) == 0 {
		return r, err
	}
	if err != nil {
		return r, err
	}
	for _, respToErr := range t.protosToErrs {
		if err = respToErr(r); err != nil {
			if errors.Is(err, errors.ErrPermanent) {
				return r, err
			}
		}
	}
	return r, err
}
