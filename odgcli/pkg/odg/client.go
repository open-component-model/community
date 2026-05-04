package odg

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/eko/gocache/lib/v4/cache"
	"github.com/eko/gocache/lib/v4/marshaler"
	bigcachestore "github.com/eko/gocache/store/bigcache/v4"

	"github.com/open-component-model/community/odgcli/pkg/github"
)

// Client provides access to the Delivery Service API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	cache      *marshaler.Marshaler
}

// clientConfig holds configurable settings for the Client.
type clientConfig struct {
	httpClient *http.Client
	timeout    time.Duration
	cacheTTL   time.Duration
}

// Option configures the Client.
type Option func(*clientConfig)

// WithHTTPClient sets a custom *http.Client. If set, WithTimeout is ignored.
func WithHTTPClient(c *http.Client) Option {
	return func(cfg *clientConfig) {
		cfg.httpClient = c
	}
}

// WithTimeout sets the HTTP client timeout. Default: 30s.
func WithTimeout(d time.Duration) Option {
	return func(cfg *clientConfig) {
		cfg.timeout = d
	}
}

// WithCacheTTL sets how long responses are cached. Default: 5m.
func WithCacheTTL(d time.Duration) Option {
	return func(cfg *clientConfig) {
		cfg.cacheTTL = d
	}
}

// NewClient authenticates against the Delivery Service and returns an
// authenticated Client.
//
// API: GET /auth
func NewClient(ctx context.Context, baseURL string, ghClient *github.Client, opts ...Option) (*Client, error) {
	cfg := &clientConfig{
		timeout:  30 * time.Second,
		cacheTTL: 5 * time.Minute,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	httpClient := cfg.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: cfg.timeout}
	}

	reqURL, err := url.JoinPath(baseURL, "auth")
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	params := url.Values{}
	params.Add("api_url", ghClient.GetURL())
	params.Add("access_token", ghClient.GetToken())

	fullURL := reqURL + "?" + params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	// Extract cookies
	var bearerToken string
	for _, cookie := range resp.Cookies() {
		switch cookie.Name {
		case "bearer_token":
			bearerToken = cookie.Value
		}
	}

	if bearerToken == "" {
		return nil, fmt.Errorf("bearer_token cookie not found in response")
	}

	// instantiate cache
	bigcacheCfg := bigcache.DefaultConfig(cfg.cacheTTL)
	bigcacheCfg.Verbose = false
	bigcacheClient, err := bigcache.New(ctx, bigcacheCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize cache: %w", err)
	}
	bigcacheStore := bigcachestore.NewBigcache(bigcacheClient)
	cacheManager := cache.New[any](bigcacheStore)

	// Initializes marshaler
	marshal := marshaler.New(cacheManager)

	return &Client{
		baseURL:    baseURL,
		token:      bearerToken,
		httpClient: httpClient,
		cache:      marshal,
	}, nil
}

func (c *Client) makeAuthenticatedRequest(ctx context.Context, method, reqURL string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return c.httpClient.Do(req)
}

// checkResponse checks whether the API response indicates an error. If so, it
// reads and drains the response body (ensuring the TCP connection can be reused)
// and returns a structured *APIError. Returns nil for 2xx status codes.
func checkResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := io.ReadAll(resp.Body)
	return &APIError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       body,
	}
}

// QueryMetadataBySearchExpressionRaw executes a single-page artefact-metadata
// search. Criteria are combined with AND across types and OR within the same
// field/attr. Pass the NextCursor from a previous response as the cursor
// parameter to retrieve the next page, or nil for the first page.
//
// For most use cases, prefer QueryMetadataBySearchExpression which handles
// pagination automatically via an iterator.
//
// API: POST /artefacts/metadata/query/by-search-expression
func (c *Client) QueryMetadataBySearchExpressionRaw(ctx context.Context, criteria []MetadataQueryCriterion, limit int, sort []MetadataQuerySort, cursor *MetadataQueryCursor) (*MetadataQueryResponse, error) {
	reqURL, err := url.JoinPath(c.baseURL, "artefacts", "metadata", "query", "by-search-expression")
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	body := MetadataQueryRequest{
		Criteria: criteria,
		Limit:    limit,
		Sort:     sort,
		Cursor:   cursor,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	resp, err := c.makeAuthenticatedRequest(ctx, http.MethodPost, reqURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, err
	}

	var result MetadataQueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// QueryMetadataBySearchExpression returns an iterator that transparently pages
// through all results matching the given criteria. Pages are fetched lazily as
// the caller consumes items. Use break to stop early and avoid unnecessary API
// calls.
//
// API: POST /artefacts/metadata/query/by-search-expression
func (c *Client) QueryMetadataBySearchExpression(ctx context.Context, criteria []MetadataQueryCriterion, pageSize int, sort []MetadataQuerySort) iter.Seq2[MetadataQueryItem, error] {
	return func(yield func(MetadataQueryItem, error) bool) {
		var cursor *MetadataQueryCursor
		for {
			resp, err := c.QueryMetadataBySearchExpressionRaw(ctx, criteria, pageSize, sort, cursor)
			if err != nil {
				yield(MetadataQueryItem{}, err)
				return
			}
			for _, item := range resp.Items {
				if !yield(item, nil) {
					return
				}
			}
			if resp.NextCursor == nil || len(resp.Items) == 0 {
				return
			}
			cursor = resp.NextCursor
		}
	}
}

// GetComplianceSummary returns the most critical severity for artefact-metadata
// types across all component dependencies. Compliance summaries contain
// severities and scan-statuses for artefact-metadata types.
//
// API: GET /components/compliance-summary
func (c *Client) GetComplianceSummary(ctx context.Context, componentName, componentVersion string) (*ComplianceSummaryResponse, error) {
	reqURL, err := url.JoinPath(c.baseURL, "components", "compliance-summary")
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	params := url.Values{}
	params.Add("component_name", componentName)
	params.Add("version", componentVersion)

	fullURL := reqURL + "?" + params.Encode()

	resp, err := c.makeAuthenticatedRequest(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, err
	}

	var result ComplianceSummaryResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetResponsiblesOptions configures the GetResponsibles request.
type GetResponsiblesOptions struct {
	// Version of the component. Default: "greatest".
	Version string
	// Raw returns raw label data if true. Default: false.
	Raw bool
	// IgnoreCache bypasses the server-side cache if true. Default: false.
	IgnoreCache bool
}

// GetResponsibles returns the user identities responsible for a given component.
// Pass nil for opts to use defaults (version=greatest, raw=false, ignore_cache=false).
//
// API: GET /ocm/component/responsibles
func (c *Client) GetResponsibles(ctx context.Context, componentName string, opts *GetResponsiblesOptions) ([]Responsible, error) {
	if opts == nil {
		opts = &GetResponsiblesOptions{}
	}
	if opts.Version == "" {
		opts.Version = "greatest"
	}

	reqURL, err := url.JoinPath(c.baseURL, "ocm", "component", "responsibles")
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	params := url.Values{}
	params.Add("component_name", componentName)
	params.Add("version", opts.Version)
	params.Add("raw", fmt.Sprintf("%t", opts.Raw))
	params.Add("ignore_cache", fmt.Sprintf("%t", opts.IgnoreCache))

	fullURL := reqURL + "?" + params.Encode()

	resp, err := c.makeAuthenticatedRequest(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, err
	}

	var result ResponsiblesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	var responsibles []Responsible
	for _, group := range result.Responsibles {
		responsibles = append(responsibles, group...)
	}

	return responsibles, nil
}

// GetRescorings calculates vulnerability rescoring proposals based on
// cve-categorisation and cve-rescoring-ruleset for a given artefact.
// Results are cached for 5 minutes.
//
// API: GET /rescore
func (c *Client) GetRescorings(ctx context.Context, artefact Artefact) ([]Finding, error) {
	findings := make([]Finding, 0)
	_, err := c.cache.Get(ctx, cacheKeyForArtefactFindings(artefact), &findings)
	if err == nil {
		return findings, nil
	} else if !errors.Is(err, bigcache.ErrEntryNotFound) {
		return findings, fmt.Errorf("failed to get findings from cache: %w", err)
	}

	reqURL, err := url.JoinPath(c.baseURL, "rescore")
	if err != nil {
		return nil, fmt.Errorf("failed to construct URL: %w", err)
	}

	params := url.Values{}
	params.Add("componentName", artefact.ComponentName)
	params.Add("componentVersion", artefact.ComponentVersion)
	params.Add("artefactKind", artefact.Kind)
	params.Add("artefactName", artefact.Info.Name)
	params.Add("artefactVersion", artefact.Info.Version)
	params.Add("artefactType", artefact.Info.Type)

	if len(artefact.Info.ExtraID) > 0 {
		extraIDJSON, err := json.Marshal(artefact.Info.ExtraID)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal artefactExtraId: %w", err)
		}
		params.Add("artefactExtraId", string(extraIDJSON))
	}

	// TODO(future): also handle license or malware findings
	params.Add("type", "finding/vulnerability")

	fullURL := reqURL + "?" + params.Encode()

	resp, err := c.makeAuthenticatedRequest(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to perform request: %w", err)
	}
	defer resp.Body.Close()

	if err := checkResponse(resp); err != nil {
		return nil, err
	}

	var result []Finding
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if err := c.cache.Set(ctx, cacheKeyForArtefactFindings(artefact), &result); err != nil {
		return nil, fmt.Errorf("failed to cache findings: %w", err)
	}

	return result, nil
}

func cacheKeyForArtefactFindings(artefact Artefact) string {
	extraID, err := json.Marshal(artefact.Info.ExtraID)
	if err != nil {
		extraID = []byte("unknown")
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s:%s:%s",
		artefact.ComponentName,
		artefact.ComponentVersion,
		artefact.Kind,
		artefact.Info.Name,
		artefact.Info.Version,
		artefact.Info.Type,
		string(extraID),
		"findings",
	)
}
