package xhs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultBaseURL = "https://adapi.xiaohongshu.com"

type Option func(*Client)

func WithHTTPClient(httpClient *http.Client) Option {
	return func(client *Client) {
		client.httpClient = httpClient
	}
}

func WithBaseURL(baseURL string) Option {
	return func(client *Client) {
		client.baseURL = strings.TrimRight(baseURL, "/")
	}
}

type Client struct {
	appID      int64
	secret     string
	baseURL    string
	httpClient *http.Client
	now        func() time.Time
}

func NewClient(appID int64, secret string, options ...Option) (*Client, error) {
	if appID <= 0 {
		return nil, errors.New("XHS Spotlight app ID must be positive")
	}
	if strings.TrimSpace(secret) == "" {
		return nil, errors.New("XHS Spotlight secret is required")
	}
	client := &Client{
		appID:      appID,
		secret:     secret,
		baseURL:    defaultBaseURL,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		now:        time.Now,
	}
	for _, option := range options {
		option(client)
	}
	if client.httpClient == nil {
		return nil, errors.New("XHS Spotlight HTTP client is nil")
	}
	if client.baseURL == "" {
		return nil, errors.New("XHS Spotlight base URL is empty")
	}
	return client, nil
}

type Advertiser struct {
	ID   int64  `json:"advertiser_id"`
	Name string `json:"advertiser_name"`
}

type Token struct {
	AccessToken           string       `json:"access_token"`
	AccessTokenExpiresIn  int64        `json:"access_token_expires_in"`
	RefreshToken          string       `json:"refresh_token"`
	RefreshTokenExpiresIn int64        `json:"refresh_token_expires_in"`
	UserID                string       `json:"user_id"`
	AppID                 int64        `json:"app_id"`
	AdvertiserID          int64        `json:"advertiser_id,omitempty"`
	ApprovalRoleType      int          `json:"approval_role_type"`
	RoleType              int          `json:"role_type"`
	PlatformType          int          `json:"platform_type"`
	ApprovalAdvertisers   []Advertiser `json:"approval_advertisers"`
	Scope                 string       `json:"scope,omitempty"`
	CorporationName       string       `json:"corporation_name,omitempty"`
	VirtualSellerID       string       `json:"virtual_seller_id,omitempty"`
	CreateTime            int64        `json:"create_time,omitempty"`
	UpdateTime            int64        `json:"update_time,omitempty"`
	AcquiredAt            time.Time    `json:"acquired_at"`
	AccessTokenExpiresAt  time.Time    `json:"access_token_expires_at"`
	RefreshTokenExpiresAt time.Time    `json:"refresh_token_expires_at"`
}

type tokenRequest struct {
	AppID    int64  `json:"app_id"`
	Secret   string `json:"secret"`
	AuthCode string `json:"auth_code"`
}

type refreshTokenRequest struct {
	AppID        int64  `json:"app_id"`
	Secret       string `json:"secret"`
	RefreshToken string `json:"refresh_token"`
}

type tokenEnvelope struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Message string `json:"msg"`
	Data    Token  `json:"data"`
}

func (client *Client) ExchangeToken(ctx context.Context, authCode string) (Token, error) {
	authCode = strings.TrimSpace(authCode)
	if authCode == "" {
		return Token{}, errors.New("XHS Spotlight auth code is required")
	}
	return client.requestToken(ctx, "/api/open/oauth2/access_token", tokenRequest{
		AppID: client.appID, Secret: client.secret, AuthCode: authCode,
	})
}

func (client *Client) RefreshToken(ctx context.Context, refreshToken string) (Token, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return Token{}, errors.New("XHS Spotlight refresh token is required")
	}
	return client.requestToken(ctx, "/api/open/oauth2/refresh_token", refreshTokenRequest{
		AppID: client.appID, Secret: client.secret, RefreshToken: refreshToken,
	})
}

func (client *Client) requestToken(ctx context.Context, path string, payload any) (Token, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Token{}, fmt.Errorf("encode XHS Spotlight token request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return Token{}, fmt.Errorf("create XHS Spotlight token request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("request XHS Spotlight token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
		return Token{}, fmt.Errorf("request XHS Spotlight token: HTTP %d", resp.StatusCode)
	}

	var envelope tokenEnvelope
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 4<<20))
	if err := decoder.Decode(&envelope); err != nil {
		return Token{}, fmt.Errorf("decode XHS Spotlight token response: %w", err)
	}
	if !envelope.Success || envelope.Code != 0 {
		message := strings.TrimSpace(envelope.Message)
		if message == "" {
			message = "unknown error"
		}
		return Token{}, fmt.Errorf("XHS Spotlight token API: code=%d message=%s", envelope.Code, message)
	}
	if envelope.Data.AccessToken == "" || envelope.Data.RefreshToken == "" {
		return Token{}, errors.New("XHS Spotlight token API returned empty access or refresh token")
	}
	if envelope.Data.AccessTokenExpiresIn <= 0 || envelope.Data.RefreshTokenExpiresIn <= 0 {
		return Token{}, errors.New("XHS Spotlight token API returned invalid token expiration")
	}

	now := client.now().UTC()
	envelope.Data.AcquiredAt = now
	envelope.Data.AccessTokenExpiresAt = now.Add(time.Duration(envelope.Data.AccessTokenExpiresIn) * time.Second)
	envelope.Data.RefreshTokenExpiresAt = now.Add(time.Duration(envelope.Data.RefreshTokenExpiresIn) * time.Second)
	return envelope.Data, nil
}
