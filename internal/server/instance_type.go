package server

import (
	"context"
	"time"

	"github.com/gogo/status"
	"google.golang.org/grpc/codes"

	cwssaws "github.com/NVIDIA/infra-controller/rest-api/workflow-schema/schema/site-agent/workflows/v1"
)

var defaultInstanceTypeIDs = []string{
	"dgx-h100-8x",
	"dgx-h100-4x",
	"hgx-h100-8x",
}

func (f *NICoServerImpl) loadDefaultInstanceTypes() {
	for _, id := range defaultInstanceTypeIDs {
		if _, exists := f.it[id]; exists {
			continue
		}
		f.it[id] = newDefaultInstanceType(id)
	}
}

func newDefaultInstanceType(id string) *cwssaws.InstanceType {
	created := time.Now().UTC().Format(time.RFC3339)
	return &cwssaws.InstanceType{
		Id:         id,
		Attributes: &cwssaws.InstanceTypeAttributes{},
		Version:    "1",
		CreatedAt:  &created,
	}
}

func (f *NICoServerImpl) CreateInstanceType(ctx context.Context, req *cwssaws.CreateInstanceTypeRequest) (*cwssaws.CreateInstanceTypeResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	id := req.GetId()
	if id == "" {
		return nil, status.Errorf(codes.InvalidArgument, "instance type id is required")
	}

	if _, exists := f.it[id]; exists {
		return nil, status.Errorf(codes.AlreadyExists, "InstanceType with ID %q already exists", id)
	}

	attrs := req.GetInstanceTypeAttributes()
	if attrs == nil {
		attrs = &cwssaws.InstanceTypeAttributes{}
	}

	created := time.Now().UTC().Format(time.RFC3339)
	it := &cwssaws.InstanceType{
		Id:         id,
		Attributes: attrs,
		Metadata:   req.GetMetadata(),
		Version:    "1",
		CreatedAt:  &created,
	}
	f.it[id] = it

	return &cwssaws.CreateInstanceTypeResponse{InstanceType: it}, nil
}

func (f *NICoServerImpl) DeleteInstanceType(ctx context.Context, req *cwssaws.DeleteInstanceTypeRequest) (*cwssaws.DeleteInstanceTypeResponse, error) {
	if req == nil || req.GetId() == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	if _, ok := f.it[req.GetId()]; !ok {
		return nil, status.Errorf(codes.NotFound, "InstanceType with ID %q not found", req.GetId())
	}

	delete(f.it, req.GetId())
	return &cwssaws.DeleteInstanceTypeResponse{}, nil
}

func (f *NICoServerImpl) FindInstanceTypesByIds(ctx context.Context, req *cwssaws.FindInstanceTypesByIdsRequest) (*cwssaws.FindInstanceTypesByIdsResponse, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	res := make([]*cwssaws.InstanceType, 0, len(req.GetInstanceTypeIds()))
	for _, id := range req.GetInstanceTypeIds() {
		if it, ok := f.it[id]; ok {
			res = append(res, it)
		}
	}
	return &cwssaws.FindInstanceTypesByIdsResponse{InstanceTypes: res}, nil
}
