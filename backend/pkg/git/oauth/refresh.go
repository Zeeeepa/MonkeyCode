package oauth

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ── Token 响应结构体 ────────────────────────────────────────────

// GitlabTokenResponse GitLab OAuth Token 响应
type GitlabTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	CreatedAt    int64  `json:"created_at"`
	Scope        string `json:"scope"`
}

func (r *GitlabTokenResponse) ExpiresAt() int64 {
	base := r.CreatedAt
	if base <= 0 {
		base = time.Now().Unix()
	}
	return base + int64(r.ExpiresIn)
}

// GiteaTokenResponse Gitea OAuth Token 响应
type GiteaTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (r *GiteaTokenResponse) ExpiresAt() int64 {
	return time.Now().Unix() + int64(r.ExpiresIn)
}

// GiteeTokenResponse Gitee OAuth Token 响应
type GiteeTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
	CreatedAt    int64  `json:"created_at"`
}

func (r *GiteeTokenResponse) ExpiresAt() int64 {
	if r.CreatedAt > 0 && r.ExpiresIn > 0 {
		return r.CreatedAt + int64(r.ExpiresIn)
	}
	return time.Now().Unix() + int64(r.ExpiresIn)
}

// ── 刷新函数 ────────────────────────────────────────────────────

// RefreshGitlab 刷新 GitLab OAuth access_token
func RefreshGitlab(baseURL, clientID, clientSecret, refreshToken string, proxies ...string) (*GitlabTokenResponse, error) {
	params := url.Values{}
	params.Add("grant_type", "refresh_token")
	params.Add("refresh_token", refreshToken)
	params.Add("client_id", clientID)
	params.Add("client_secret", clientSecret)

	tokenURL := fmt.Sprintf("%s/oauth/token", strings.TrimSuffix(baseURL, "/"))

	result, err := fetchWithProxyAndBody[GitlabTokenResponse](
		http.MethodPost, tokenURL,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
		strings.NewReader(params.Encode()), proxies...,
	)
	if err != nil {
		return nil, fmt.Errorf("refresh gitlab token: %w", err)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("refreshed gitlab access token is empty")
	}
	return result, nil
}

// RefreshGitea 刷新 Gitea OAuth access_token
func RefreshGitea(baseURL, clientID, clientSecret, refreshToken string, proxies ...string) (*GiteaTokenResponse, error) {
	params := url.Values{}
	params.Add("grant_type", "refresh_token")
	params.Add("refresh_token", refreshToken)
	params.Add("client_id", clientID)
	params.Add("client_secret", clientSecret)

	tokenURL := fmt.Sprintf("%s/login/oauth/access_token", strings.TrimSuffix(baseURL, "/"))

	result, err := fetchWithProxyAndBody[GiteaTokenResponse](
		http.MethodPost, tokenURL,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Accept": "application/json"},
		strings.NewReader(params.Encode()), proxies...,
	)
	if err != nil {
		return nil, fmt.Errorf("refresh gitea token: %w", err)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("refreshed gitea access token is empty")
	}
	return result, nil
}

// CodeupTokenResponse 阿里云云效 OAuth Token 响应
type CodeupTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	Scope        string `json:"scope"`
}

func (r *CodeupTokenResponse) ExpiresAt() int64 {
	return time.Now().Unix() + int64(r.ExpiresIn)
}

// RefreshCodeup 刷新云效 OAuth access_token
//
// 默认 token endpoint 走阿里云账号通用网关：https://account.aliyun.com/oauth2/v1/token
// 如有定制（如专有云）可通过 tokenURL 显式覆盖
func RefreshCodeup(tokenURL, clientID, clientSecret, refreshToken string, proxies ...string) (*CodeupTokenResponse, error) {
	if tokenURL == "" {
		tokenURL = "https://account.aliyun.com/oauth2/v1/token"
	}
	params := url.Values{}
	params.Add("grant_type", "refresh_token")
	params.Add("refresh_token", refreshToken)
	params.Add("client_id", clientID)
	params.Add("client_secret", clientSecret)

	result, err := fetchWithProxyAndBody[CodeupTokenResponse](
		http.MethodPost, tokenURL,
		map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Accept": "application/json"},
		strings.NewReader(params.Encode()), proxies...,
	)
	if err != nil {
		return nil, fmt.Errorf("refresh codeup token: %w", err)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("refreshed codeup access token is empty")
	}
	return result, nil
}

// RefreshGitee 刷新 Gitee OAuth access_token（无需 client 凭证）
func RefreshGitee(refreshToken string) (*GiteeTokenResponse, error) {
	params := url.Values{}
	params.Add("grant_type", "refresh_token")
	params.Add("refresh_token", refreshToken)

	result, err := fetchWithProxyAndBody[GiteeTokenResponse](
		http.MethodPost, "https://gitee.com/oauth/token",
		map[string]string{"Content-Type": "application/x-www-form-urlencoded"},
		strings.NewReader(params.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("refresh gitee token: %w", err)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("refreshed gitee access token is empty")
	}
	return result, nil
}
