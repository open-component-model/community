package odg

import (
	"fmt"
	"time"
)

// APIError represents an error response from the Delivery Service API.
// Use errors.As to inspect the status code or response body.
type APIError struct {
	StatusCode int
	Status     string
	Body       []byte
}

func (e *APIError) Error() string {
	if len(e.Body) > 0 {
		return fmt.Sprintf("API error %s: %s", e.Status, string(e.Body))
	}
	return fmt.Sprintf("API error %s", e.Status)
}

// ComplianceSummaryResponse represents the full API response
type ComplianceSummaryResponse struct {
	ComplianceSummary []ComplianceSummaryItem `json:"complianceSummary"`
}

// ComplianceSummaryItem represents a single compliance summary item
type ComplianceSummaryItem struct {
	ComponentID ComponentID     `json:"componentId"`
	Entries     []Entry         `json:"entries"`
	Artefacts   []ArtefactEntry `json:"artefacts"`
}

// ComponentID represents a component identifier
type ComponentID struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Entry represents a compliance entry
type Entry struct {
	Type           string `json:"type"`
	Source         string `json:"source"`
	Categorisation string `json:"categorisation"`
	Value          int    `json:"value"`
	ScanStatus     string `json:"scanStatus"`
}

// ArtefactInfo represents the inner artefact details
type ArtefactInfo struct {
	Name    string                 `json:"artefact_name"`
	Version string                 `json:"artefact_version"`
	Type    string                 `json:"artefact_type"`
	ExtraID map[string]interface{} `json:"artefact_extra_id"`
}

// Artefact represents an artefact with its metadata
type Artefact struct {
	ComponentName    string                   `json:"component_name"`
	ComponentVersion string                   `json:"component_version"`
	Kind             string                   `json:"artefact_kind"`
	Info             ArtefactInfo             `json:"artefact"`
	References       []map[string]interface{} `json:"references"`
}

// ArtefactEntry represents an artefact with its compliance entries
type ArtefactEntry struct {
	Artefact Artefact `json:"artefact"`
	Entries  []Entry  `json:"entries"`
}

// Responsible represents a responsible person for a component
type Responsible struct {
	Source         string `json:"source"`
	Type           string `json:"type"`
	Username       string `json:"username"`
	GithubHostname string `json:"github_hostname"`
	Email          string `json:"email"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	OriginType     string `json:"origin_type"`
}

// ResponsiblesStatus represents a status message in the responsibles response
type ResponsiblesStatus struct {
	Type string `json:"type"`
	Msg  string `json:"msg"`
}

// ResponsiblesResponse represents the response from the responsibles endpoint
type ResponsiblesResponse struct {
	Responsibles [][]Responsible      `json:"responsibles"`
	Statuses     []ResponsiblesStatus `json:"statuses"`
}

// Sprint represents sprint information
type Sprint struct {
	Name    string `json:"name"`
	EndDate string `json:"end_date"`
}

// Finding represents a single finding from the rescore endpoint
type Finding struct {
	Finding              FindingDetails        `json:"finding"`
	FindingType          string                `json:"finding_type"`
	Severity             string                `json:"severity"`
	MatchingRules        []string              `json:"matching_rules"`
	ApplicableRescorings []ApplicableRescoring `json:"applicable_rescorings"`
	DiscoveryDate        string                `json:"discovery_date"`
	DueDate              *string               `json:"due_date"`
	Sprint               *Sprint               `json:"sprint"`
}

// FindingDetails represents the finding details
type FindingDetails struct {
	Severity        string           `json:"severity"`
	PackageName     string           `json:"package_name"`
	PackageVersions []string         `json:"package_versions"`
	CVE             string           `json:"cve"`
	CVSSv3Score     float64          `json:"cvss_v3_score"`
	CVSS            string           `json:"cvss"`
	Summary         string           `json:"summary"`
	URLs            []string         `json:"urls"`
	FilesystemPaths []FilesystemPath `json:"filesystem_paths"`
}

// ApplicableRescoring represents a rescoring that applies to a finding
type ApplicableRescoring struct {
	Artefact              RescoringArtefact `json:"artefact"`
	Meta                  RescoringMeta     `json:"meta"`
	Data                  RescoringData     `json:"data"`
	DiscoveryDate         *string           `json:"discovery_date"`
	AllowedProcessingTime *string           `json:"allowed_processing_time"`
	ID                    string            `json:"id"`
}

// PathElement represents a single path element in filesystem_paths
type PathElement struct {
	Path string `json:"path"`
	Type string `json:"type"`
}

// FilesystemPath represents a filesystem path with digest
type FilesystemPath struct {
	Path   []PathElement `json:"path"`
	Digest string        `json:"digest"`
}

// RescoringArtefact represents the artefact in a rescoring
type RescoringArtefact struct {
	ComponentName    string                   `json:"component_name"`
	ComponentVersion *string                  `json:"component_version"`
	Artefact         RescoringArtefactInfo    `json:"artefact"`
	ArtefactKind     string                   `json:"artefact_kind"`
	References       []map[string]interface{} `json:"references"`
}

// RescoringArtefactInfo represents the inner artefact info in a rescoring
type RescoringArtefactInfo struct {
	ArtefactName    *string                `json:"artefact_name"`
	ArtefactType    string                 `json:"artefact_type"`
	ArtefactVersion *string                `json:"artefact_version"`
	ArtefactExtraID map[string]interface{} `json:"artefact_extra_id"`
}

// RescoringMeta represents the meta information of a rescoring
type RescoringMeta struct {
	Datasource   string  `json:"datasource"`
	Type         string  `json:"type"`
	CreationDate string  `json:"creation_date"`
	LastUpdate   string  `json:"last_update"`
	Responsibles *string `json:"responsibles"`
	AssigneeMode *string `json:"assignee_mode"`
}

// RescoringUser represents the user who created the rescoring
type RescoringUser struct {
	Username       string `json:"username"`
	Type           string `json:"type"`
	GithubHostname string `json:"github_hostname,omitempty"`
	Email          string `json:"email,omitempty"`
	Firstname      string `json:"firstname,omitempty"`
	Lastname       string `json:"lastname,omitempty"`
}

// RescoringFinding represents the finding reference in a rescoring
type RescoringFinding struct {
	PackageName string `json:"package_name"`
	CVE         string `json:"cve"`
}

// RescoringData represents the data of a rescoring
type RescoringData struct {
	Finding               RescoringFinding `json:"finding"`
	ReferencedType        string           `json:"referenced_type"`
	Severity              string           `json:"severity"`
	User                  RescoringUser    `json:"user"`
	MatchingRules         []string         `json:"matching_rules"`
	Comment               string           `json:"comment"`
	AllowedProcessingTime *string          `json:"allowed_processing_time"`
	DueDate               *string          `json:"due_date"`
}

// Comment represents a rescoring comment extracted from the metadata query API.
type Comment struct {
	Author           string    `json:"author"`
	Content          string    `json:"content"`
	CreatedAt        time.Time `json:"created_at"`
	ComponentName    string    `json:"component_name"`
	ComponentVersion string    `json:"component_version"`
	ArtefactName     string    `json:"artefact_name"`
	ArtefactVersion  string    `json:"artefact_version"`
	Severity         string    `json:"severity"`
}

// MetadataQueryCriterion represents a search criterion for the metadata query.
type MetadataQueryCriterion struct {
	Type  string `json:"type"`
	Attr  string `json:"attr"`
	Op    string `json:"op"`
	Value string `json:"value"`
}

// MetadataQuerySort represents a sort order for the metadata query.
type MetadataQuerySort struct {
	Field string `json:"field"`
	Order string `json:"order"`
}

// MetadataQueryCursor represents the cursor for seek-based pagination.
// It must contain all sort fields.
type MetadataQueryCursor struct {
	CreationDate string `json:"meta.creation_date"`
	ID           string `json:"id"`
}

// MetadataQueryRequest represents the request body for the metadata query endpoint.
type MetadataQueryRequest struct {
	Criteria []MetadataQueryCriterion `json:"criteria"`
	Limit    int                      `json:"limit"`
	Sort     []MetadataQuerySort      `json:"sort"`
	Cursor   *MetadataQueryCursor     `json:"cursor,omitempty"`
}

// MetadataQueryArtefact is like Artefact but with a nullable component_version.
type MetadataQueryArtefact struct {
	ComponentName    string                   `json:"component_name"`
	ComponentVersion *string                  `json:"component_version"`
	Info             ArtefactInfo             `json:"artefact"`
	Kind             string                   `json:"artefact_kind"`
	References       []map[string]interface{} `json:"references"`
}

// MetadataQueryMeta represents the meta field in a metadata query response item.
type MetadataQueryMeta struct {
	Datasource   string  `json:"datasource"`
	Type         string  `json:"type"`
	CreationDate string  `json:"creation_date"`
	LastUpdate   string  `json:"last_update"`
	Responsibles *string `json:"responsibles"`
	AssigneeMode *string `json:"assignee_mode"`
}

// MetadataQueryItem represents a single item in the metadata query response.
type MetadataQueryItem struct {
	Artefact              MetadataQueryArtefact `json:"artefact"`
	Meta                  MetadataQueryMeta     `json:"meta"`
	Data                  RescoringData         `json:"data"`
	DiscoveryDate         *string               `json:"discovery_date"`
	AllowedProcessingTime *string               `json:"allowed_processing_time"`
}

// MetadataQueryResponse represents the response from the metadata query endpoint.
type MetadataQueryResponse struct {
	Items      []MetadataQueryItem  `json:"items"`
	NextCursor *MetadataQueryCursor `json:"nextCursor,omitempty"`
}
