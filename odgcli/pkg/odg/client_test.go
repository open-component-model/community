package odg

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-component-model/community/odgcli/pkg/github"
)

// newTestServer creates an httptest.Server that mimics the Delivery Service API,
// returning fixture data for known endpoints.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/auth":
			http.SetCookie(w, &http.Cookie{Name: "bearer_token", Value: "test-token"})
			w.WriteHeader(http.StatusOK)

		case r.URL.Path == "/components/compliance-summary":
			serveFixture(w, "testdata/compliance_summary.json")

		case r.URL.Path == "/ocm/component/responsibles":
			serveFixture(w, "testdata/responsibles.json")

		case r.URL.Path == "/rescore":
			serveFixture(w, "testdata/rescorings.json")

		case r.URL.Path == "/artefacts/metadata/query/by-search-expression":
			// Check if cursor is present in request body for pagination test.
			var body MetadataQueryRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			if body.Cursor != nil {
				serveFixture(w, "testdata/metadata_query_page2.json")
			} else {
				serveFixture(w, "testdata/metadata_query.json")
			}

		default:
			http.NotFound(w, r)
		}
	}))
}

func serveFixture(w http.ResponseWriter, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, "fixture not found: "+path, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// newTestClient creates a Client pointed at the given test server.
func newTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()

	ghClient := github.NewClient("https://api.github.com", "fake-token")
	client, err := NewClient(context.Background(), serverURL, ghClient)
	require.NoError(t, err)
	return client
}

func TestNewClient(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	ghClient := github.NewClient("https://api.github.com", "fake-token")
	client, err := NewClient(context.Background(), srv.URL, ghClient)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, "test-token", client.token)
	assert.Equal(t, srv.URL, client.baseURL)
}

func TestNewClient_MissingToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 200 but no bearer_token cookie.
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ghClient := github.NewClient("https://api.github.com", "fake-token")
	_, err := NewClient(context.Background(), srv.URL, ghClient)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bearer_token cookie not found")
}

func TestNewClient_AuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid token"}`))
	}))
	defer srv.Close()

	ghClient := github.NewClient("https://api.github.com", "fake-token")
	_, err := NewClient(context.Background(), srv.URL, ghClient)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "bearer_token cookie not found")
}

func TestGetComplianceSummary(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	client := newTestClient(t, srv.URL)

	result, err := client.GetComplianceSummary(context.Background(), "ocm.software/ocmcli", "greatest")

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.GreaterOrEqual(t, len(result.ComplianceSummary), 1)

	first := result.ComplianceSummary[0]
	assert.Equal(t, "ocm.software/ocmcli", first.ComponentID.Name)
	assert.NotEmpty(t, first.ComponentID.Version)
	assert.NotEmpty(t, first.Entries)
	assert.NotEmpty(t, first.Artefacts)
}

func TestGetResponsibles(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	client := newTestClient(t, srv.URL)

	responsibles, err := client.GetResponsibles(context.Background(), "ocm.software/ocmcli", nil)

	require.NoError(t, err)
	assert.Len(t, responsibles, 2)
	assert.Equal(t, "johndoe", responsibles[0].Username)
	assert.Equal(t, "janedoe", responsibles[1].Username)
	assert.Equal(t, "john.doe@example.com", responsibles[0].Email)
}

func TestGetResponsibles_WithOptions(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	client := newTestClient(t, srv.URL)

	responsibles, err := client.GetResponsibles(context.Background(), "ocm.software/ocmcli", &GetResponsiblesOptions{
		Version:     "1.0.0",
		IgnoreCache: true,
	})

	require.NoError(t, err)
	assert.Len(t, responsibles, 2)
}

func TestGetRescorings(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	client := newTestClient(t, srv.URL)

	artefact := Artefact{
		ComponentName:    "ocm.software/ocmcli",
		ComponentVersion: "0.40.0",
		Kind:             "resource",
		Info: ArtefactInfo{
			Name:    "ocmcli-image",
			Version: "0.40.0",
			Type:    "ociImage",
		},
	}

	findings, err := client.GetRescorings(context.Background(), artefact)

	require.NoError(t, err)
	assert.Len(t, findings, 3)
	assert.NotEmpty(t, findings[0].Finding.CVE)
	assert.NotEmpty(t, findings[0].Severity)
}

func TestGetRescorings_Cached(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/auth":
			http.SetCookie(w, &http.Cookie{Name: "bearer_token", Value: "test-token"})
		case r.URL.Path == "/rescore":
			callCount.Add(1)
			serveFixture(w, "testdata/rescorings.json")
		}
	}))
	defer srv.Close()
	client := newTestClient(t, srv.URL)

	artefact := Artefact{
		ComponentName:    "ocm.software/ocmcli",
		ComponentVersion: "0.40.0",
		Kind:             "resource",
		Info: ArtefactInfo{
			Name:    "ocmcli-image",
			Version: "0.40.0",
			Type:    "ociImage",
		},
	}

	// First call hits the server.
	_, err := client.GetRescorings(context.Background(), artefact)
	require.NoError(t, err)

	// Second call should be served from cache.
	_, err = client.GetRescorings(context.Background(), artefact)
	require.NoError(t, err)

	assert.Equal(t, int32(1), callCount.Load(), "expected only 1 server call due to caching")
}

func TestQueryMetadataBySearchExpressionRaw(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	client := newTestClient(t, srv.URL)

	result, err := client.QueryMetadataBySearchExpressionRaw(context.Background(), []MetadataQueryCriterion{
		{Type: "artefact-metadata", Attr: "type", Op: "eq", Value: "rescorings"},
	}, 2, []MetadataQuerySort{
		{Field: "meta.creation_date", Order: "desc"},
		{Field: "id", Order: "desc"},
	}, nil)

	require.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.NotNil(t, result.NextCursor)
	assert.NotEmpty(t, result.NextCursor.ID)
	assert.NotEmpty(t, result.NextCursor.CreationDate)
}

func TestQueryMetadataBySearchExpression_Pagination(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	client := newTestClient(t, srv.URL)

	var items []MetadataQueryItem
	for item, err := range client.QueryMetadataBySearchExpression(context.Background(), []MetadataQueryCriterion{
		{Type: "artefact-metadata", Attr: "type", Op: "eq", Value: "rescorings"},
	}, 2, []MetadataQuerySort{
		{Field: "meta.creation_date", Order: "desc"},
		{Field: "id", Order: "desc"},
	}) {
		require.NoError(t, err)
		items = append(items, item)
	}

	// Page 1 has 2 items, page 2 has 1 item = 3 total.
	assert.Len(t, items, 3)
	// First items from page 1.
	assert.Equal(t, "acme.org/sovereign/postgres", items[0].Artefact.ComponentName)
	// Last item from page 2.
	assert.Equal(t, "acme.org/sovereign/redis", items[2].Artefact.ComponentName)
}

func TestQueryMetadataBySearchExpression_EarlyBreak(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/auth":
			http.SetCookie(w, &http.Cookie{Name: "bearer_token", Value: "test-token"})
		case r.URL.Path == "/artefacts/metadata/query/by-search-expression":
			callCount.Add(1)
			serveFixture(w, "testdata/metadata_query.json")
		}
	}))
	defer srv.Close()
	client := newTestClient(t, srv.URL)

	count := 0
	for _, err := range client.QueryMetadataBySearchExpression(context.Background(), []MetadataQueryCriterion{
		{Type: "artefact-metadata", Attr: "type", Op: "eq", Value: "rescorings"},
	}, 2, []MetadataQuerySort{
		{Field: "meta.creation_date", Order: "desc"},
		{Field: "id", Order: "desc"},
	}) {
		require.NoError(t, err)
		count++
		break // Stop after first item.
	}

	assert.Equal(t, 1, count)
	// Only 1 API call was made (didn't fetch page 2).
	assert.Equal(t, int32(1), callCount.Load())
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/auth":
			http.SetCookie(w, &http.Cookie{Name: "bearer_token", Value: "test-token"})
		case r.URL.Path == "/components/compliance-summary":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"error": "insufficient permissions"}`))
		}
	}))
	defer srv.Close()
	client := newTestClient(t, srv.URL)

	_, err := client.GetComplianceSummary(context.Background(), "test", "1.0.0")

	require.Error(t, err)
	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, 403, apiErr.StatusCode)
	assert.Contains(t, string(apiErr.Body), "insufficient permissions")
}
