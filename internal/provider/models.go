package provider

import "time"

type Repository struct {
	Provider string `json:"provider"`
	Host     string `json:"host"`
	Owner    string `json:"owner"`
	Name     string `json:"name"`
}

func (r Repository) FullName() string { return r.Owner + "/" + r.Name }

type User struct {
	Login string `json:"login"`
	Name  string `json:"name,omitempty"`
}

type PullRequest struct {
	Number       int        `json:"number"`
	ID           string     `json:"id,omitempty"`
	Title        string     `json:"title"`
	Body         string     `json:"body,omitempty"`
	State        string     `json:"state"`
	Draft        bool       `json:"draft"`
	URL          string     `json:"url"`
	Author       User       `json:"author"`
	BaseBranch   string     `json:"base_branch"`
	HeadBranch   string     `json:"head_branch"`
	HeadSHA      string     `json:"head_sha"`
	Mergeable    *bool      `json:"mergeable,omitempty"`
	MergeState   string     `json:"merge_state,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	MergedAt     *time.Time `json:"merged_at,omitempty"`
	ChangedFiles int        `json:"changed_files,omitempty"`
	Additions    int        `json:"additions,omitempty"`
	Deletions    int        `json:"deletions,omitempty"`
}

type Review struct {
	ID          string     `json:"id"`
	Author      User       `json:"author"`
	State       string     `json:"state"`
	Body        string     `json:"body,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	CommitID    string     `json:"commit_id,omitempty"`
}

type ReviewComment struct {
	ID         string    `json:"id"`
	DatabaseID int64     `json:"database_id,omitempty"`
	Author     User      `json:"author"`
	Body       string    `json:"body"`
	Path       string    `json:"path,omitempty"`
	Line       *int      `json:"line,omitempty"`
	URL        string    `json:"url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type ReviewThread struct {
	ID       string          `json:"id"`
	Resolved bool            `json:"resolved"`
	Outdated bool            `json:"outdated"`
	Path     string          `json:"path,omitempty"`
	Line     *int            `json:"line,omitempty"`
	Comments []ReviewComment `json:"comments"`
}

type Check struct {
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Conclusion  string     `json:"conclusion,omitempty"`
	URL         string     `json:"url,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type Status struct {
	Context     string `json:"context"`
	State       string `json:"state"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
}

type Checks struct {
	Overall  string   `json:"overall"`
	Checks   []Check  `json:"checks"`
	Statuses []Status `json:"statuses"`
}

type Inspection struct {
	Repository  Repository     `json:"repository"`
	PullRequest PullRequest    `json:"pull_request"`
	Reviews     []Review       `json:"reviews"`
	Threads     []ReviewThread `json:"threads"`
	Checks      Checks         `json:"checks"`
}

type EditInput struct {
	Title *string `json:"title,omitempty"`
	Body  *string `json:"body,omitempty"`
	State *string `json:"state,omitempty"`
}

type MergeInput struct {
	Method      string `json:"method"`
	Title       string `json:"title,omitempty"`
	Message     string `json:"message,omitempty"`
	ExpectedSHA string `json:"expected_sha,omitempty"`
}

type MergeResult struct {
	Merged  bool   `json:"merged"`
	SHA     string `json:"sha,omitempty"`
	Message string `json:"message"`
}

type MutationPreview struct {
	Provider string      `json:"provider"`
	Action   string      `json:"action"`
	Target   string      `json:"target"`
	Input    interface{} `json:"input,omitempty"`
}
