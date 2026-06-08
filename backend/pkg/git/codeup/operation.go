package codeup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chaitin/MonkeyCode/backend/domain"
	"github.com/chaitin/MonkeyCode/backend/pkg/request"
)

// Tree 实现 GitClienter 接口
//
// Codeup 文件树接口：GET /oapi/v1/codeup/organizations/{orgId}/repositories/{repoIdent}/files/tree
// query: ref=branch, path=subdir, type=DIRECT|RECURSIVE
func (c *Codeup) Tree(ctx context.Context, opts *domain.TreeOptions) (*domain.GetRepoTreeResp, error) {
	orgID, identity := c.splitRepoCtx(opts.Owner, opts.Repo)
	if orgID == "" || identity == "" {
		return nil, fmt.Errorf("codeup tree: missing org or repo identity")
	}

	path := c.repoPath(orgID, identity) + "/files/tree"
	query := request.Query{}
	if opts.Ref != "" {
		query["ref"] = opts.Ref
	}
	if opts.Path != "" {
		query["path"] = opts.Path
	}
	if opts.Recursive {
		query["type"] = "RECURSIVE"
	} else {
		query["type"] = "DIRECT"
	}

	nodes, err := request.Get[[]*TreeNode](c.client, ctx, path,
		request.WithHeader(c.authHeader(opts.Token)),
		request.WithQuery(query),
	)
	if err != nil {
		return nil, fmt.Errorf("get codeup tree: %w", err)
	}

	entries := make([]*domain.TreeEntry, 0, len(*nodes))
	for _, n := range *nodes {
		entries = append(entries, &domain.TreeEntry{
			Mode: codeupTypeToMode(n.Type),
			Name: n.Name,
			Path: n.Path,
			Sha:  n.ID,
		})
	}
	return &domain.GetRepoTreeResp{
		Entries: entries,
		SHA:     opts.Ref,
	}, nil
}

// Blob 实现 GitClienter 接口
//
// Codeup 文件内容接口：GET /oapi/v1/codeup/organizations/{orgId}/repositories/{repoIdent}/files/blobs
// query: filePath, ref
func (c *Codeup) Blob(ctx context.Context, opts *domain.BlobOptions) (*domain.GetBlobResp, error) {
	orgID, identity := c.splitRepoCtx(opts.Owner, opts.Repo)
	if orgID == "" || identity == "" {
		return nil, fmt.Errorf("codeup blob: missing org or repo identity")
	}

	apiPath := c.repoPath(orgID, identity) + "/files/blobs"
	query := request.Query{"filePath": opts.Path}
	if opts.Ref != "" {
		query["ref"] = opts.Ref
	}

	blob, err := request.Get[FileBlob](c.client, ctx, apiPath,
		request.WithHeader(c.authHeader(opts.Token)),
		request.WithQuery(query),
	)
	if err != nil {
		return nil, fmt.Errorf("get codeup blob: %w", err)
	}

	var content []byte
	switch strings.ToLower(blob.Encoding) {
	case "base64":
		cleaned := strings.ReplaceAll(blob.Content, "\n", "")
		decoded, decErr := base64.StdEncoding.DecodeString(cleaned)
		if decErr != nil {
			return nil, fmt.Errorf("decode base64: %w", decErr)
		}
		content = decoded
	default:
		content = []byte(blob.Content)
	}

	return &domain.GetBlobResp{
		Content:  content,
		IsBinary: isBinaryContent(content),
		Sha:      firstNonEmpty(blob.BlobID, blob.LastCommitID, blob.CommitID),
		Size:     blob.Size,
	}, nil
}

// Logs 实现 GitClienter 接口
//
// Codeup commits 接口：GET /oapi/v1/codeup/organizations/{orgId}/repositories/{repoIdent}/commits
// query: refName, path, page, perPage
func (c *Codeup) Logs(ctx context.Context, opts *domain.LogsOptions) (*domain.GetGitLogsResp, error) {
	orgID, identity := c.splitRepoCtx(opts.Owner, opts.Repo)
	if orgID == "" || identity == "" {
		return nil, fmt.Errorf("codeup logs: missing org or repo identity")
	}

	limit := opts.Limit
	if limit <= 0 {
		limit = 100
	}
	page := (opts.Offset / limit) + 1

	apiPath := c.repoPath(orgID, identity) + "/commits"
	query := request.Query{
		"page":    fmt.Sprintf("%d", page),
		"perPage": fmt.Sprintf("%d", limit),
	}
	if opts.Ref != "" {
		query["refName"] = opts.Ref
	}
	if opts.Path != "" {
		query["path"] = opts.Path
	}

	commits, err := request.Get[[]*CommitItem](c.client, ctx, apiPath,
		request.WithHeader(c.authHeader(opts.Token)),
		request.WithQuery(query),
	)
	if err != nil {
		return nil, fmt.Errorf("list codeup commits: %w", err)
	}

	list := *commits
	skip := opts.Offset % limit
	if skip > 0 {
		if skip >= len(list) {
			list = nil
		} else {
			list = list[skip:]
		}
	}

	entries := make([]*domain.GitCommitEntry, 0, len(list))
	for _, c := range list {
		entry := &domain.GitCommitEntry{Commit: &domain.GitCommit{
			Sha:        c.ID,
			Message:    firstNonEmpty(c.Message, c.Title),
			ParentShas: append([]string(nil), c.ParentIDs...),
		}}
		entry.Commit.Author = &domain.GitUser{
			Name:  firstNonEmpty(c.AuthorName, userField(c.Author, "name")),
			Email: firstNonEmpty(c.AuthorEmail, userField(c.Author, "email")),
			When:  parseCodeupTime(firstNonEmpty(c.AuthoredDate, userField(c.Author, "date"))),
		}
		entry.Commit.Committer = &domain.GitUser{
			Name:  firstNonEmpty(c.CommitterName, userField(c.Committer, "name"), entry.Commit.Author.Name),
			Email: firstNonEmpty(c.CommitterEmail, userField(c.Committer, "email"), entry.Commit.Author.Email),
			When:  parseCodeupTime(firstNonEmpty(c.CommittedDate, userField(c.Committer, "date"))),
		}
		entries = append(entries, entry)
	}
	return &domain.GetGitLogsResp{
		Count:   len(entries),
		Entries: entries,
	}, nil
}

// Archive 实现 GitClienter 接口
//
// Codeup 归档接口：GET /oapi/v1/codeup/organizations/{orgId}/repositories/{repoIdent}/archive
// query: sha=branch, format=tar.gz|zip
func (c *Codeup) Archive(ctx context.Context, opts *domain.ArchiveOptions) (*domain.GetRepoArchiveResp, error) {
	orgID, identity := c.splitRepoCtx(opts.Owner, opts.Repo)
	if orgID == "" || identity == "" {
		return nil, fmt.Errorf("codeup archive: missing org or repo identity")
	}

	ref := opts.Ref
	if ref == "" {
		ref = "master"
	}
	apiURL := fmt.Sprintf("%s%s/archive?sha=%s&format=tar.gz",
		c.BaseURL(), c.repoPath(orgID, identity), url.QueryEscape(ref))
	resp, err := c.rawRequest(ctx, "GET", apiURL, opts.Token)
	if err != nil {
		return nil, err
	}
	return &domain.GetRepoArchiveResp{
		ContentLength: resp.ContentLength,
		ContentType:   "application/gzip",
		Reader:        resp.Body,
	}, nil
}

// Branches 实现 GitClienter 接口
func (c *Codeup) Branches(ctx context.Context, opts *domain.BranchesOptions) ([]*domain.BranchInfo, error) {
	orgID, identity := c.splitRepoCtx(opts.Owner, opts.Repo)
	if orgID == "" || identity == "" {
		return nil, fmt.Errorf("codeup branches: missing org or repo identity")
	}

	page := opts.Page
	if page <= 0 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = 50
	}
	if perPage > 100 {
		perPage = 100
	}

	apiPath := c.repoPath(orgID, identity) + "/branches"
	query := request.Query{
		"page":    fmt.Sprintf("%d", page),
		"perPage": fmt.Sprintf("%d", perPage),
	}
	branches, err := request.Get[[]*Branch](c.client, ctx, apiPath,
		request.WithHeader(c.authHeader(opts.Token)),
		request.WithQuery(query),
	)
	if err != nil {
		return nil, fmt.Errorf("list codeup branches: %w", err)
	}
	result := make([]*domain.BranchInfo, 0, len(*branches))
	for _, b := range *branches {
		result = append(result, &domain.BranchInfo{Name: b.Name})
	}
	return result, nil
}

// CreateWebhook 实现 GitClienter 接口
//
// Codeup webhook 接口：POST /oapi/v1/codeup/organizations/{orgId}/repositories/{repoIdent}/webhooks
func (c *Codeup) CreateWebhook(ctx context.Context, opts *domain.CreateWebhookOptions) error {
	orgID, identity, err := ParseRepoPath(opts.RepoURL)
	if err != nil {
		return err
	}

	apiPath := c.repoPath(orgID, identity) + "/webhooks"
	payload := map[string]any{
		"url":                opts.WebhookURL,
		"secretToken":        opts.SecretToken,
		"pushEvents":         true,
		"mergeRequestsEvents": true,
		"tagPushEvents":      true,
		"description":        "MonkeyCode webhook",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	apiURL := c.BaseURL() + apiPath
	resp, err := c.rawJSON(ctx, "POST", apiURL, opts.Token, body)
	if err != nil {
		return fmt.Errorf("create codeup webhook: %w", err)
	}
	resp.Body.Close()
	return nil
}

// DeleteWebhook 实现 GitClienter 接口
func (c *Codeup) DeleteWebhook(ctx context.Context, opts *domain.WebhookOptions) error {
	orgID, identity, err := ParseRepoPath(opts.RepoURL)
	if err != nil {
		return err
	}

	apiPath := c.repoPath(orgID, identity) + "/webhooks"
	// 分页查找匹配 webhook 后删除
	for page := 1; page <= 20; page++ {
		query := request.Query{
			"page":    fmt.Sprintf("%d", page),
			"perPage": "100",
		}
		hooks, err := request.Get[[]*WebhookItem](c.client, ctx, apiPath,
			request.WithHeader(c.authHeader(opts.Token)),
			request.WithQuery(query),
		)
		if err != nil {
			return fmt.Errorf("list codeup webhooks: %w", err)
		}
		if hooks == nil || len(*hooks) == 0 {
			return nil
		}
		for _, h := range *hooks {
			if h.URL != opts.WebhookURL {
				continue
			}
			delURL := fmt.Sprintf("%s%s/%d", c.BaseURL(), apiPath, h.ID)
			resp, derr := c.rawRequest(ctx, "DELETE", delURL, opts.Token)
			if derr != nil {
				return derr
			}
			resp.Body.Close()
			return nil
		}
		if len(*hooks) < 100 {
			return nil
		}
	}
	return nil
}

func (c *Codeup) rawJSON(ctx context.Context, method, fullURL, token string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-yunxiao-token", token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		buf, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("codeup %s returned %d: %s", method, resp.StatusCode, parseError(buf))
	}
	return resp, nil
}

// splitRepoCtx 还原 codeup 的 orgId + repo identity。
//
// 约定：orgId 总是来自客户端构造时注入（来自 db.GitIdentity.OrganizationID）；
// Owner / Repo 是 group/repo 形式的 identity 组成部分。
func (c *Codeup) splitRepoCtx(owner, repo string) (string, string) {
	owner = strings.TrimSpace(owner)
	repo = strings.TrimSpace(repo)
	switch {
	case owner == "" && repo == "":
		return c.orgID, ""
	case owner == "":
		return c.orgID, repo
	case repo == "":
		return c.orgID, owner
	default:
		return c.orgID, owner + "/" + repo
	}
}

func codeupTypeToMode(t string) int {
	switch strings.ToLower(t) {
	case "tree", "dir", "directory":
		return 4
	default:
		return 1
	}
}

func isBinaryContent(content []byte) bool {
	if len(content) == 0 {
		return false
	}
	check := content
	if len(check) > 8000 {
		check = check[:8000]
	}
	return bytes.Contains(check, []byte{0})
}

func userField(u *CommitUser, key string) string {
	if u == nil {
		return ""
	}
	switch key {
	case "name":
		return u.Name
	case "email":
		return u.Email
	case "date":
		return u.Date
	}
	return ""
}

func parseCodeupTime(s string) int64 {
	if s == "" {
		return 0
	}
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05+08:00",
		"2006-01-02 15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Unix()
		}
	}
	return 0
}
