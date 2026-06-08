package codeup

// 默认 OpenAPI 域名（公网公共站）
const DefaultOpenAPIHost = "openapi-rdc.aliyuncs.com"

// 默认 Git clone 域名
const DefaultGitHost = "codeup.aliyun.com"

// Repository 仓库信息
type Repository struct {
	ID            int64    `json:"id,omitempty"`
	Name          string   `json:"name,omitempty"`
	Path          string   `json:"path,omitempty"`
	NameWithNs    string   `json:"nameWithNamespace,omitempty"`
	PathWithNs    string   `json:"pathWithNamespace,omitempty"`
	Description   string   `json:"description,omitempty"`
	VisibilityLvl int      `json:"visibilityLevel,omitempty"` // 0 私有 / 10 内部 / 20 公开
	WebURL        string   `json:"webUrl,omitempty"`
	HTTPCloneURL  string   `json:"httpCloneUrl,omitempty"`
	SSHCloneURL   string   `json:"sshCloneUrl,omitempty"`
	DefaultBranch string   `json:"defaultBranch,omitempty"`
	Permissions   []string `json:"permissions,omitempty"` // 权限列表
	Archived      bool     `json:"archived,omitempty"`
}

// Branch 分支
type Branch struct {
	Name      string         `json:"name,omitempty"`
	Protected bool           `json:"protected,omitempty"`
	Commit    *CommitSummary `json:"commit,omitempty"`
}

// CommitSummary 分支挂载的提交摘要
type CommitSummary struct {
	ID        string `json:"id,omitempty"`
	ShortID   string `json:"shortId,omitempty"`
	Title     string `json:"title,omitempty"`
	AuthoredAt string `json:"authoredDate,omitempty"`
}

// TreeNode 文件树节点
type TreeNode struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"` // tree / blob
	Path string `json:"path,omitempty"`
	Mode string `json:"mode,omitempty"`
}

// FileBlob 文件内容
type FileBlob struct {
	FileName    string `json:"fileName,omitempty"`
	FilePath    string `json:"filePath,omitempty"`
	Size        int    `json:"size,omitempty"`
	Encoding    string `json:"encoding,omitempty"` // base64 / text
	Content     string `json:"content,omitempty"`
	CommitID    string `json:"commitId,omitempty"`
	LastCommitID string `json:"lastCommitId,omitempty"`
	Ref         string `json:"ref,omitempty"`
	BlobID      string `json:"blobId,omitempty"`
}

// CommitUser commit 中的作者/提交者
type CommitUser struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Date  string `json:"date,omitempty"`
}

// CommitItem 完整提交信息
type CommitItem struct {
	ID             string      `json:"id,omitempty"`
	ShortID        string      `json:"shortId,omitempty"`
	Title          string      `json:"title,omitempty"`
	Message        string      `json:"message,omitempty"`
	ParentIDs      []string    `json:"parentIds,omitempty"`
	AuthorName     string      `json:"authorName,omitempty"`
	AuthorEmail    string      `json:"authorEmail,omitempty"`
	AuthoredDate   string      `json:"authoredDate,omitempty"`
	CommitterName  string      `json:"committerName,omitempty"`
	CommitterEmail string      `json:"committerEmail,omitempty"`
	CommittedDate  string      `json:"committedDate,omitempty"`
	Author         *CommitUser `json:"author,omitempty"`
	Committer      *CommitUser `json:"committer,omitempty"`
	WebURL         string      `json:"webUrl,omitempty"`
}

// OrganizationItem 云效组织条目（来自 /oapi/v1/platform/organizations）
type OrganizationItem struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	CreatorID   string `json:"creatorId,omitempty"`
	DefaultRole string `json:"defaultRole,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	UpdateAt    string `json:"updateAt,omitempty"`
}

// WebhookItem 仓库 webhook
type WebhookItem struct {
	ID          int64  `json:"id,omitempty"`
	URL         string `json:"url,omitempty"`
	Description string `json:"description,omitempty"`
}

// errorResponse 云效错误响应
type errorResponse struct {
	ErrorCode    string `json:"errorCode,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
	Message      string `json:"message,omitempty"`
	RequestID    string `json:"requestId,omitempty"`
}
