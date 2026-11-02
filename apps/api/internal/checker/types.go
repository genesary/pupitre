package checker

// EvaluateRequest is sent by course-service to POST /evaluate.
// The project template has already been resolved ({{ .Username }} replaced).
// Raw Rego text is not accepted: the policy is fetched directly from the
// course git repository so an attacker who can reach this endpoint cannot
// inject arbitrary Rego.
type EvaluateRequest struct {
	Username    string   `json:"username"`
	Project     string   `json:"project"`
	Files       []string `json:"files"`
	PolicySrc   string   `json:"policySrc"`             // git repository URL
	PolicyRef   string   `json:"policyRef"`             // branch / tag / commit
	PolicyPath  string   `json:"policyPath"`            // relative path to the .rego file
	PolicyToken *string  `json:"policyToken,omitempty"` // optional git authentication token
}

// EvaluateResponse is returned to course-service.
type EvaluateResponse struct {
	Allow      bool     `json:"allow"`
	Violations []string `json:"violations"`
}

// GitLabState is the JSON input passed to OPA. Field names are snake_case
// and documented in docs/interactive-labs.md as the policy input schema;
// course authors write Rego policies against these exact names, so the
// wire format cannot be changed without breaking existing course policies.
type GitLabState struct {
	Project       *projectInfo      `json:"project"`
	MergedMRCount int               `json:"mergedMrCount"`
	OpenMRs       []openMRInfo      `json:"openMrs"`
	Files         map[string]string `json:"files"`
}

// projectInfo is the GitLab project metadata exposed to OPA policies.
type projectInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"pathWithNamespace"`
	DefaultBranch string `json:"defaultBranch"`
}

// openMRInfo is a single open merge request exposed to OPA policies.
type openMRInfo struct {
	IID            int64        `json:"iid"`
	Title          string       `json:"title"`
	SourceBranch   string       `json:"sourceBranch"`
	PipelineStatus string       `json:"pipelineStatus"`
	Commits        []commitInfo `json:"commits"`
}

// commitInfo is a single commit exposed to OPA policies.
type commitInfo struct {
	Message string `json:"message"`
}
