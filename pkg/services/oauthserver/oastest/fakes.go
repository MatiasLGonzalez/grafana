package oastest

import (
	"context"
	"net/http"

	"github.com/grafana/grafana/pkg/services/oauthserver"
	"github.com/grafana/grafana/pkg/services/serviceauth"
	"gopkg.in/square/go-jose.v2"
)

type FakeService struct {
	ExpectedClient *oauthserver.ExternalService
	ExpectedKey    *jose.JSONWebKey
	ExpectedErr    error
}

var _ oauthserver.OAuth2Server = &FakeService{}

func (s *FakeService) SaveExternalService(ctx context.Context, cmd *serviceauth.ExternalServiceRegistration) (*serviceauth.ExternalServiceDTO, error) {
	return s.ExpectedClient.ToDTO(nil), s.ExpectedErr
}

func (s *FakeService) GetExternalService(ctx context.Context, id string) (*oauthserver.ExternalService, error) {
	return s.ExpectedClient, s.ExpectedErr
}

func (s *FakeService) HandleTokenRequest(rw http.ResponseWriter, req *http.Request) {}

func (s *FakeService) HandleIntrospectionRequest(rw http.ResponseWriter, req *http.Request) {}
