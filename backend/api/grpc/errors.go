package grpc

import (
	"errors"

	"github.com/avnpl/go-march/utils/customErrors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func grpcError(err error) error {
	if err == nil {
		return nil
	}
	if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
		return err
	}

	switch {
	case errors.Is(err, customErrors.RecordNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, customErrors.Conflict):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, customErrors.InvalidRequest),
		errors.Is(err, customErrors.OutOfStock),
		errors.Is(err, customErrors.IncorrectAmount),
		errors.Is(err, customErrors.FailedTransaction):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
