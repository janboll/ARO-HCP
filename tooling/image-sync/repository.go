package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/containers/azcontainerregistry"
)

// Registry is the interface for accessing image repositories
type Registry interface {
	GetTags(context.Context, string) ([]string, error)
}

// AuthedTransport is a http.RoundTripper that adds an Authorization header
type AuthedTransport struct {
	Key     string
	Wrapped http.RoundTripper
}

// RoundTrip implements http.RoundTripper and sets Authorization header
func (t *AuthedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", t.Key)
	return t.Wrapped.RoundTrip(req)
}

// QuayRegistry implements Quay Repository access
type QuayRegistry struct {
	httpclient   *http.Client
	baseUrl      string
	numberOftags int
}

// NewQuayRegistry creates a new QuayRegistry access client
func NewQuayRegistry(cfg *SyncConfig, bearerToken string) *QuayRegistry {
	secretBytes, err := os.ReadFile(cfg.QuaySecretFile)
	if err != nil {
		Log().Fatalf("failed to read secret file: %v", err)
	}
	return &QuayRegistry{
		httpclient: &http.Client{Timeout: time.Duration(cfg.RequestTimeout) * time.Second,
			Transport: &AuthedTransport{
				Key:     "Bearer " + string(secretBytes),
				Wrapped: http.DefaultTransport,
			},
		},
		baseUrl:      "https://quay.io",
		numberOftags: cfg.NumberOfTags,
	}
}

type TagsResponse struct {
	Tags          []Tags
	Page          int
	HasAdditional bool
}

type Tags struct {
	Name string
}

// GetTags returns the tags for the given image
func (q *QuayRegistry) GetTags(ctx context.Context, image string) ([]string, error) {
	// Todo pagination
	Log().Debugw("Getting tags for image", "image", image)
	path := fmt.Sprintf("%s/api/v1/repository/%s/tag/", q.baseUrl, image)
	req, err := http.NewRequestWithContext(ctx, "GET", path, nil)
	Log().Debugw("Sending request", "path", path)
	if err != nil {
		return nil, err
	}
	resp, err := q.httpclient.Do(req)
	if err != nil {
		return nil, err
	}
	Log().Debugw("Got response", "statuscode", resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var tagsResponse TagsResponse
	json.Unmarshal(body, &tagsResponse)

	var tags []string

	for _, tag := range tagsResponse.Tags {
		if tag.Name == "latest" {
			continue
		}
		tags = append(tags, tag.Name)
		if len(tags) == q.numberOftags-1 {
			break
		}
	}

	return tags, nil
}

// AzureContainerRegistry implements ACR Repository access
type AzureContainerRegistry struct {
	acrName      string
	credential   *azidentity.DefaultAzureCredential
	acrClient    *azcontainerregistry.Client
	numberOfTags int
	tenantId     string
}

// NewAzureContainerRegistry creates a new AzureContainerRegistry access client
func NewAzureContainerRegistry(cfg *SyncConfig) *AzureContainerRegistry {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		Log().Fatalf("failed to obtain a credential: %v", err)
	}

	client, err := azcontainerregistry.NewClient(fmt.Sprintf("https://%s", cfg.AcrRegistry), cred, nil)
	if err != nil {
		Log().Fatalf("failed to create client: %v", err)
	}

	return &AzureContainerRegistry{
		acrName:      cfg.AcrRegistry,
		acrClient:    client,
		credential:   cred,
		numberOfTags: cfg.NumberOfTags,
		tenantId:     cfg.TenantId,
	}
}

type AuthSecret struct {
	RefreshToken string `json:"refresh_token"`
}

func (a *AzureContainerRegistry) GetPullSecret(ctx context.Context) (*AuthSecret, error) {
	accessToken, err := a.credential.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{"https://management.core.windows.net//.default"}})

	path := fmt.Sprintf("https://%s/oauth2/exchange/", a.acrName)

	form := url.Values{}
	form.Add("grant_type", "access_token")
	form.Add("service", a.acrName)
	form.Add("tenant", a.tenantId)
	form.Add("access_token", accessToken.Token)

	Log().Debugw("Sending request", "path", path)
	req, err := http.NewRequestWithContext(ctx, "POST", path, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var authSecret AuthSecret

	json.Unmarshal(body, &authSecret)

	return &authSecret, nil
}

// EnsureRepositoryExists ensures that the repository exists
func (a *AzureContainerRegistry) RepositoryExists(ctx context.Context, repository string) (bool, error) {

	pager := a.acrClient.NewListRepositoriesPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return false, fmt.Errorf("failed to advance page: %v", err)
		}
		for _, v := range page.Repositories.Names {
			if *v == repository {
				return true, nil
			}
		}
	}

	return false, nil
}

// GetTags returns the tags in the given repository
func (a *AzureContainerRegistry) GetTags(ctx context.Context, repository string) ([]string, error) {
	var tags []string

	pager := a.acrClient.NewListTagsPager(repository, &azcontainerregistry.ClientListTagsOptions{OrderBy: to.Ptr(azcontainerregistry.ArtifactTagOrderByLastUpdatedOnDescending)})
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to advance page: %v", err)
		}
		for _, v := range page.Tags {
			if *v.Name == "latest" {
				continue
			}
			tags = append(tags, *v.Name)
		}
		if len(tags) == a.numberOfTags-1 {
			break
		}
	}

	return tags, nil
}
