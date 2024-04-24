package golive

//go:generate go run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen --config=golive-gen.yaml tem-api.yaml

import (
	"context"
	"encoding/base64"
	"fmt"
	"k8s.io/klog/v2"
	"net/http"
)

const (
	DEFAULT_GOLIVE_URL = "https://golive.apwide.net/api"
)

type GoliveConfig struct {
	Url      string
	Username string
	Password string
	Token    string
}

type securityProvider struct {
	authorizationHeader string
}

func newSecurityProvider(ctx context.Context, cfg GoliveConfig) (provider securityProvider) {
	logger := klog.FromContext(ctx)
	if cfg.Token != "" {
		logger.V(2).Info("Golive Authentication is Bearer")
		provider.authorizationHeader = fmt.Sprintf("Bearer %s", cfg.Token)
	} else if cfg.Username != "" {
		logger.V(2).Info("Golive Authentication is Basic")
		creds := base64.StdEncoding.EncodeToString([]byte(cfg.Username + ":" + cfg.Password))
		provider.authorizationHeader = fmt.Sprintf("Basic %s", creds)
	} else {
		logger.V(2).Info("No Golive Authentication provided")
	}
	return provider
}

func (s *securityProvider) Intercept(_ context.Context, req *http.Request) error {
	req.Header.Set("Authorization", s.authorizationHeader)
	return nil
}

func Golive(ctx context.Context, config GoliveConfig) (*ClientWithResponses, error) {
	url := DEFAULT_GOLIVE_URL
	if config.Url != "" {
		url = config.Url
	}
	klog.FromContext(ctx).V(2).Info(fmt.Sprintf("Golive URL is '%s'", url))
	provider := newSecurityProvider(ctx, config)
	if provider.authorizationHeader != "" {
		return NewClientWithResponses(url, WithRequestEditorFn(provider.Intercept))
	}
	client, err := NewClientWithResponses(url)
	if err != nil {
		return nil, err
	}
	err = client.Test(ctx)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// TODO call to GoliveInfo (ClientProduct) or call /statuses
func (c *ClientWithResponses) Test(ctx context.Context) error {
	result, err := c.GetApplicationsWithResponse(ctx, nil)
	if err != nil {
		return err
	}
	if result.StatusCode() != 200 {
		return fmt.Errorf("HTTP Error %d on contacting Golive: %q", result.StatusCode(), string(result.Body))
	}
	return nil
}
