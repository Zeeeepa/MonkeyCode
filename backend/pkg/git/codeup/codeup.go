// Package codeup 提供阿里云云效 Codeup 客户端。
//
// 鉴权：HTTP Header `x-yunxiao-token: <PAT 或 OAuth access_token>`，统一走云效 OpenAPI（公共站
// openapi-rdc.aliyuncs.com）。绝大多数接口需要 organizationId 路径参数。
package codeup

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chaitin/MonkeyCode/backend/domain"
	"github.com/chaitin/MonkeyCode/backend/pkg/request"
)

// Codeup 客户端
type Codeup struct {
	client    *request.Client
	logger    *slog.Logger
	openapi   string // OpenAPI host，如 openapi-rdc.aliyuncs.com
	gitHost   string // Git clone 域名，如 codeup.aliyun.com
	scheme    string
	orgID     string // 默认组织 ID；当 GitIdentity 已存储 OrganizationID 时注入
}

// NewCodeup 创建 Codeup 客户端。
//   - openapiBase: OpenAPI 域名（包含 scheme），空则使用默认 https://openapi-rdc.aliyuncs.com
//   - orgID: 组织 ID，可为空（CheckPAT / UserInfo / 自动解析时无需提前知道）
func NewCodeup(openapiBase, orgID string, logger *slog.Logger) *Codeup {
	scheme, host := normalizeBase(openapiBase, DefaultOpenAPIHost)
	return &Codeup{
		logger:  logger.With("module", "codeup"),
		openapi: host,
		scheme:  scheme,
		gitHost: DefaultGitHost,
		orgID:   orgID,
		client: request.NewClient(
			scheme,
			host,
			30*time.Second,
		),
	}
}

// BaseURL 返回 OpenAPI base URL
func (c *Codeup) BaseURL() string {
	return c.scheme + "://" + c.openapi
}

// OrgID 返回当前默认组织 ID（可能为空）
func (c *Codeup) OrgID() string { return c.orgID }

func (c *Codeup) authHeader(token string) request.Header {
	return request.Header{"x-yunxiao-token": token}
}

func normalizeBase(input, fallback string) (scheme, host string) {
	scheme, host = "https", fallback
	if input == "" {
		return
	}
	if strings.HasPrefix(input, "http://") {
		scheme = "http"
		host = strings.TrimPrefix(input, "http://")
	} else if strings.HasPrefix(input, "https://") {
		host = strings.TrimPrefix(input, "https://")
	} else {
		host = input
	}
	host = strings.TrimSuffix(host, "/")
	return
}

// ParseRepoPath 从仓库 URL 解析 orgId 和 repo 标识（groupPath/repoName）。
// 支持以下形式：
//   https://codeup.aliyun.com/{orgId}/{group}/{repo}.git
//   https://codeup.aliyun.com/{orgId}/{group}/{repo}
//   git@codeup.aliyun.com:{orgId}/{group}/{repo}.git
//   https://{orgId}.codeup.aliyun.com/{group}/{repo}.git
func ParseRepoPath(repoURL string) (orgID, identity string, err error) {
	raw := strings.TrimSpace(repoURL)
	raw = strings.TrimSuffix(raw, ".git")

	// SSH 形式：git@host:org/group/repo
	if strings.HasPrefix(raw, "git@") {
		at := strings.Index(raw, "@")
		col := strings.Index(raw, ":")
		if col <= at {
			return "", "", fmt.Errorf("invalid codeup ssh url: %s", repoURL)
		}
		path := raw[col+1:]
		return splitOrgAndIdentity(path, repoURL)
	}

	u, perr := url.Parse(raw)
	if perr != nil {
		return "", "", fmt.Errorf("parse codeup url: %w", perr)
	}
	host := u.Host
	path := strings.TrimPrefix(u.Path, "/")

	// 子域名形式：{orgId}.codeup.aliyun.com/{group}/{repo}
	if idx := strings.Index(host, ".codeup."); idx > 0 {
		orgID = host[:idx]
		identity = path
		if orgID == "" || identity == "" {
			return "", "", fmt.Errorf("invalid codeup subdomain url: %s", repoURL)
		}
		return orgID, identity, nil
	}

	// 路径形式：codeup.aliyun.com/{orgId}/{group}/{repo}
	return splitOrgAndIdentity(path, repoURL)
}

func splitOrgAndIdentity(path, raw string) (string, string, error) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("invalid codeup url, expect orgId/group/repo: %s", raw)
	}
	orgID := parts[0]
	identity := strings.Join(parts[1:], "/")
	return orgID, identity, nil
}

// repoPath 返回 /oapi/v1/codeup/organizations/{orgId}/repositories/{repoIdent}
// repoIdent 既支持数字 ID 也支持 group%2Frepo 形式
func (c *Codeup) repoPath(orgID, repoIdent string) string {
	return fmt.Sprintf("/oapi/v1/codeup/organizations/%s/repositories/%s",
		url.PathEscape(orgID), url.PathEscape(repoIdent))
}

func (c *Codeup) resolveOrgID(explicit string) string {
	if explicit != "" {
		return explicit
	}
	return c.orgID
}

// GetCurrentUser 获取当前 token 对应的用户信息（含组织）
func (c *Codeup) GetCurrentUser(ctx context.Context, token string) (*CurrentUser, error) {
	user, err := request.Get[CurrentUser](c.client, ctx, "/oapi/v1/codeup/users/current",
		request.WithHeader(c.authHeader(token)))
	if err != nil {
		return nil, fmt.Errorf("get current user: %w", err)
	}
	return user, nil
}

// ListOrganizations 获取当前 token 所属的组织列表
func (c *Codeup) ListOrganizations(ctx context.Context, token string) ([]*OrganizationItem, error) {
	orgs, err := request.Get[[]*OrganizationItem](c.client, ctx, "/oapi/v1/codeup/organizations",
		request.WithHeader(c.authHeader(token)))
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	if orgs == nil {
		return nil, nil
	}
	return *orgs, nil
}

// ResolveOrgID 自动解析 token 对应的默认组织 ID（取第一个）
func (c *Codeup) ResolveOrgID(ctx context.Context, token string) (string, error) {
	if user, err := c.GetCurrentUser(ctx, token); err == nil && user.OrgID != "" {
		return user.OrgID, nil
	}
	orgs, err := c.ListOrganizations(ctx, token)
	if err != nil {
		return "", err
	}
	if len(orgs) == 0 {
		return "", fmt.Errorf("token has no organization")
	}
	return orgs[0].ID, nil
}

// GetRepoByIdentity 根据 group/repo 标识获取仓库信息
func (c *Codeup) GetRepoByIdentity(ctx context.Context, token, orgID, identity string) (*Repository, error) {
	if orgID == "" {
		return nil, fmt.Errorf("organization id is required")
	}
	path := c.repoPath(orgID, identity)
	repo, err := request.Get[Repository](c.client, ctx, path,
		request.WithHeader(c.authHeader(token)))
	if err != nil {
		return nil, fmt.Errorf("get repo: %w", err)
	}
	return repo, nil
}

// CheckPAT 校验 PAT。先解析 repoURL 得到 orgId + identity，再调 OpenAPI 验证可访问。
func (c *Codeup) CheckPAT(ctx context.Context, token, repoURL string) (bool, *domain.BindRepository, error) {
	orgID, identity, err := ParseRepoPath(repoURL)
	if err != nil {
		return false, nil, err
	}
	repo, err := c.GetRepoByIdentity(ctx, token, orgID, identity)
	if err != nil {
		return false, nil, err
	}
	if repo == nil || repo.ID == 0 {
		return false, nil, fmt.Errorf("repository not found or token has no access")
	}
	web := repo.WebURL
	if web == "" {
		web = repo.HTTPCloneURL
	}
	return true, &domain.BindRepository{
		RepoID:          fmt.Sprintf("%d", repo.ID),
		RepoName:        repo.Name,
		FullName:        firstNonEmpty(repo.PathWithNs, repo.NameWithNs, identity),
		RepoURL:         web,
		RepoDescription: repo.Description,
		IsPrivate:       repo.VisibilityLvl == 0,
		Platform:        "codeup",
	}, nil
}

// UserInfo 实现 GitClienter 接口
func (c *Codeup) UserInfo(ctx context.Context, token string) (*domain.PlatformUserInfo, error) {
	user, err := c.GetCurrentUser(ctx, token)
	if err != nil {
		return nil, err
	}
	return &domain.PlatformUserInfo{Name: firstNonEmpty(user.Username, user.Name, user.Email)}, nil
}

// Repositories 列出当前用户在组织内可访问的仓库
func (c *Codeup) Repositories(ctx context.Context, opts *domain.RepositoryOptions) ([]domain.AuthRepository, error) {
	orgID := c.resolveOrgID("")
	if orgID == "" {
		resolved, err := c.ResolveOrgID(ctx, opts.Token)
		if err != nil {
			return nil, fmt.Errorf("resolve organization: %w", err)
		}
		orgID = resolved
	}

	result := make([]domain.AuthRepository, 0, 64)
	page, perPage := 1, 100
	for {
		path := fmt.Sprintf("/oapi/v1/codeup/organizations/%s/repositories", url.PathEscape(orgID))
		query := request.Query{
			"page":    fmt.Sprintf("%d", page),
			"perPage": fmt.Sprintf("%d", perPage),
			"orderBy": "last_activity_at",
			"sort":    "desc",
		}
		repos, err := request.Get[[]*Repository](c.client, ctx, path,
			request.WithHeader(c.authHeader(opts.Token)),
			request.WithQuery(query),
		)
		if err != nil {
			return nil, fmt.Errorf("list codeup repositories: %w", err)
		}
		if repos == nil || len(*repos) == 0 {
			break
		}
		for _, r := range *repos {
			result = append(result, domain.AuthRepository{
				FullName:    firstNonEmpty(r.PathWithNs, r.NameWithNs, r.Name),
				URL:         firstNonEmpty(r.WebURL, r.HTTPCloneURL),
				Description: r.Description,
			})
		}
		if len(*repos) < perPage {
			break
		}
		page++
		if page > 50 { // 安全上限：5000 个仓库
			break
		}
	}
	return result, nil
}

// rawRequest 透传 HTTP 调用，主要用于 archive 这类返回二进制流的接口
func (c *Codeup) rawRequest(ctx context.Context, method, fullURL, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-yunxiao-token", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("codeup %s %s returned %d: %s", method, fullURL, resp.StatusCode, parseError(body))
	}
	return resp, nil
}

func parseError(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var er errorResponse
	if err := json.Unmarshal(body, &er); err == nil {
		if er.ErrorMessage != "" {
			return er.ErrorMessage
		}
		if er.Message != "" {
			return er.Message
		}
		if er.ErrorCode != "" {
			return er.ErrorCode
		}
	}
	if len(body) > 512 {
		return string(body[:512])
	}
	return string(body)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
