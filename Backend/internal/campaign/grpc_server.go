package campaign

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	catalogv1 "pulsebid/gen/pulsebid/catalog/v1"
)

// CatalogServer exposes the shared snapshot cache over internal gRPC.
type CatalogServer struct {
	catalogv1.UnimplementedCatalogServiceServer
	snapshots *SnapshotService
}

// NewCatalogServer uses the same cache as the authenticated REST endpoint.
func NewCatalogServer(snapshots *SnapshotService) *CatalogServer {
	return &CatalogServer{snapshots: snapshots}
}

// GetSnapshot implements the catalog/v1 contract without leaking storage errors.
func (s *CatalogServer) GetSnapshot(ctx context.Context, request *catalogv1.GetSnapshotRequest) (*catalogv1.GetSnapshotResponse, error) {
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	result, err := s.snapshots.GetSnapshot(ctx, request.KnownVersion)
	if err != nil {
		return nil, snapshotGRPCError(err)
	}
	return result, nil
}

func snapshotGRPCError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "snapshot deadline exceeded")
	case errors.Is(err, ErrValidation):
		return status.Error(codes.InvalidArgument, "invalid snapshot request")
	case errors.Is(err, ErrUnavailable):
		return status.Error(codes.Unavailable, "catalog unavailable")
	default:
		return status.Error(codes.Internal, "could not load catalog")
	}
}
