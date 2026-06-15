package server

import (
	"context"
	"strings"
	"time"

	"github.com/gogo/status"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"

	cwssaws "github.com/NVIDIA/infra-controller/rest-api/workflow-schema/schema/site-agent/workflows/v1"
)

func (f *NICoServerImpl) operatingSystemLookupID(config *cwssaws.InstanceConfig) string {
	if config == nil || config.Os == nil {
		return ""
	}
	if id := config.Os.GetOperatingSystemId(); id != nil && id.Value != "" {
		return id.Value
	}
	if id := config.Os.GetOsImageId(); id != nil && id.Value != "" {
		return id.Value
	}
	return ""
}

func (f *NICoServerImpl) resolveUserData(config *cwssaws.InstanceConfig) string {
	if config == nil || config.Os == nil {
		return ""
	}
	if userData := strings.TrimSpace(config.Os.GetUserData()); userData != "" {
		return userData
	}

	osID := f.operatingSystemLookupID(config)
	if osID == "" {
		return ""
	}
	os, ok := f.oss[osID]
	if !ok || os == nil {
		return ""
	}
	return strings.TrimSpace(os.GetUserData())
}

func (f *NICoServerImpl) ensureOperatingSystemStub(id string, req *cwssaws.OsImageAttributes) {
	if id == "" {
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	os := f.oss[id]
	if os == nil {
		os = &cwssaws.OperatingSystem{
			Id:     &cwssaws.OperatingSystemId{Value: id},
			Status: cwssaws.TenantState_READY,
		}
	}
	if req != nil {
		if req.TenantOrganizationId != "" {
			os.TenantOrganizationId = req.TenantOrganizationId
		}
		if req.Name != nil && *req.Name != "" {
			os.Name = *req.Name
		}
		if req.Description != nil {
			os.Description = req.Description
		}
	}
	if os.Created == "" {
		os.Created = now
	}
	os.Updated = now
	os.IsActive = true
	f.oss[id] = os
}

func operatingSystemTypeFromCreateRequest(req *cwssaws.CreateOperatingSystemRequest) cwssaws.OperatingSystemType {
	if req == nil {
		return cwssaws.OperatingSystemType_OS_TYPE_UNSPECIFIED
	}
	if req.GetIpxeTemplateId() != nil {
		return cwssaws.OperatingSystemType_OS_TYPE_TEMPLATED_IPXE
	}
	if strings.TrimSpace(req.GetIpxeScript()) != "" {
		return cwssaws.OperatingSystemType_OS_TYPE_IPXE
	}
	return cwssaws.OperatingSystemType_OS_TYPE_UNSPECIFIED
}

func operatingSystemFromCreateRequest(req *cwssaws.CreateOperatingSystemRequest) (*cwssaws.OperatingSystem, error) {
	if req == nil {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}

	id := ""
	if req.GetId() != nil && req.GetId().Value != "" {
		id = req.GetId().Value
	} else {
		id = uuid.NewString()
	}

	now := time.Now().UTC().Format(time.RFC3339)
	os := &cwssaws.OperatingSystem{
		Id:                   &cwssaws.OperatingSystemId{Value: id},
		Name:                 req.GetName(),
		Description:          req.Description,
		TenantOrganizationId: req.GetTenantOrganizationId(),
		Type:                 operatingSystemTypeFromCreateRequest(req),
		Status:               cwssaws.TenantState_READY,
		IsActive:             req.GetIsActive(),
		AllowOverride:        req.GetAllowOverride(),
		PhoneHomeEnabled:     req.GetPhoneHomeEnabled(),
		Created:              now,
		Updated:              now,
		IpxeScript:           req.IpxeScript,
		IpxeTemplateId:       req.IpxeTemplateId,
		IpxeTemplateParameters:     req.GetIpxeTemplateParameters(),
		IpxeTemplateArtifacts:      req.GetIpxeTemplateArtifacts(),
	}
	if userData := strings.TrimSpace(req.GetUserData()); userData != "" {
		os.UserData = &userData
	}
	return os, nil
}

func applyOperatingSystemUpdate(os *cwssaws.OperatingSystem, req *cwssaws.UpdateOperatingSystemRequest) {
	if os == nil || req == nil {
		return
	}
	if req.Name != nil {
		os.Name = *req.Name
	}
	if req.Description != nil {
		os.Description = req.Description
	}
	if req.IsActive != nil {
		os.IsActive = *req.IsActive
	}
	if req.AllowOverride != nil {
		os.AllowOverride = *req.AllowOverride
	}
	if req.PhoneHomeEnabled != nil {
		os.PhoneHomeEnabled = *req.PhoneHomeEnabled
	}
	if req.UserData != nil {
		userData := strings.TrimSpace(*req.UserData)
		if userData == "" {
			os.UserData = nil
		} else {
			os.UserData = &userData
		}
	}
	if req.IpxeScript != nil {
		os.IpxeScript = req.IpxeScript
	}
	if req.IpxeTemplateId != nil {
		os.IpxeTemplateId = req.IpxeTemplateId
	}
	if req.IpxeTemplateParameters != nil {
		os.IpxeTemplateParameters = req.IpxeTemplateParameters.GetItems()
	}
	if req.IpxeTemplateArtifacts != nil {
		os.IpxeTemplateArtifacts = req.IpxeTemplateArtifacts.GetItems()
	}
	if req.IpxeTemplateDefinitionHash != nil {
		os.IpxeTemplateDefinitionHash = req.IpxeTemplateDefinitionHash
	}
	os.Updated = time.Now().UTC().Format(time.RFC3339)
}

// CreateOperatingSystem stores a tenant operating system definition on the site.
func (f *NICoServerImpl) CreateOperatingSystem(ctx context.Context, req *cwssaws.CreateOperatingSystemRequest) (*cwssaws.OperatingSystem, error) {
	os, err := operatingSystemFromCreateRequest(req)
	if err != nil {
		return nil, err
	}
	if _, exists := f.oss[os.GetId().GetValue()]; exists {
		return nil, status.Errorf(codes.AlreadyExists, "OperatingSystem with ID %q already exists", os.GetId().GetValue())
	}
	f.oss[os.GetId().GetValue()] = os
	return os, nil
}

// GetOperatingSystem returns a stored operating system by id.
func (f *NICoServerImpl) GetOperatingSystem(ctx context.Context, req *cwssaws.OperatingSystemId) (*cwssaws.OperatingSystem, error) {
	if req == nil || req.Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	os, ok := f.oss[req.Value]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "OperatingSystem with ID %q not found", req.Value)
	}
	return os, nil
}

// UpdateOperatingSystem updates a stored operating system definition.
func (f *NICoServerImpl) UpdateOperatingSystem(ctx context.Context, req *cwssaws.UpdateOperatingSystemRequest) (*cwssaws.OperatingSystem, error) {
	if req == nil || req.GetId() == nil || req.GetId().Value == "" {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid request argument")
	}
	os, ok := f.oss[req.GetId().Value]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "OperatingSystem with ID %q not found", req.GetId().Value)
	}
	applyOperatingSystemUpdate(os, req)
	return os, nil
}
