package xhs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

var ErrNotAuthorized = errors.New("XHS Spotlight is not authorized")

type ManagerOption func(*TokenManager)

func WithRefreshBefore(duration time.Duration) ManagerOption {
	return func(manager *TokenManager) {
		manager.refreshBefore = duration
	}
}

func WithRefreshRetry(duration time.Duration) ManagerOption {
	return func(manager *TokenManager) {
		manager.retryInterval = duration
	}
}

func WithRefreshErrorHandler(handler func(error)) ManagerOption {
	return func(manager *TokenManager) {
		manager.onRefreshError = handler
	}
}

type TokenManager struct {
	client         *Client
	sessionPath    string
	refreshBefore  time.Duration
	retryInterval  time.Duration
	onRefreshError func(error)
	now            func() time.Time

	operationMu sync.Mutex
	mu          sync.RWMutex
	token       Token
	authorized  bool
	lastAttempt time.Time
	lastSuccess time.Time
	lastError   string
	nextRetry   time.Time
	wake        chan struct{}
}

type ManagerStatus struct {
	Authorized             bool         `json:"authorized"`
	AccessTokenValid       bool         `json:"access_token_valid"`
	RefreshTokenValid      bool         `json:"refresh_token_valid"`
	UserID                 string       `json:"user_id,omitempty"`
	AppID                  int64        `json:"app_id,omitempty"`
	AdvertiserID           int64        `json:"advertiser_id,omitempty"`
	ApprovalRoleType       int          `json:"approval_role_type,omitempty"`
	RoleType               int          `json:"role_type,omitempty"`
	PlatformType           int          `json:"platform_type,omitempty"`
	Scope                  string       `json:"scope,omitempty"`
	CorporationName        string       `json:"corporation_name,omitempty"`
	VirtualSellerID        string       `json:"virtual_seller_id,omitempty"`
	ApprovalAdvertisers    []Advertiser `json:"approval_advertisers,omitempty"`
	AccessTokenExpiresAt   time.Time    `json:"access_token_expires_at,omitempty"`
	RefreshTokenExpiresAt  time.Time    `json:"refresh_token_expires_at,omitempty"`
	LastRefreshAttemptAt   time.Time    `json:"last_refresh_attempt_at,omitempty"`
	LastRefreshSucceededAt time.Time    `json:"last_refresh_succeeded_at,omitempty"`
	LastRefreshError       string       `json:"last_refresh_error,omitempty"`
}

func NewTokenManager(client *Client, sessionPath string, options ...ManagerOption) (*TokenManager, error) {
	if client == nil {
		return nil, errors.New("XHS Spotlight client is required")
	}
	if sessionPath == "" {
		return nil, errors.New("XHS Spotlight session path is required")
	}
	manager := &TokenManager{
		client:        client,
		sessionPath:   sessionPath,
		refreshBefore: 10 * time.Minute,
		retryInterval: time.Minute,
		now:           time.Now,
		wake:          make(chan struct{}, 1),
	}
	for _, option := range options {
		option(manager)
	}
	if manager.refreshBefore <= 0 {
		return nil, errors.New("XHS Spotlight refresh-before duration must be positive")
	}
	if manager.retryInterval <= 0 {
		return nil, errors.New("XHS Spotlight refresh retry interval must be positive")
	}

	session, err := LoadSession(sessionPath)
	if err == nil {
		manager.token = session.Token
		manager.authorized = true
		return manager, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return manager, nil
	}
	return nil, err
}

func (manager *TokenManager) Authorize(ctx context.Context, authCode string) (Token, error) {
	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()

	token, err := manager.client.ExchangeToken(ctx, authCode)
	if err != nil {
		return Token{}, err
	}
	if err := SaveSession(manager.sessionPath, token); err != nil {
		return Token{}, err
	}
	manager.mu.Lock()
	manager.token = token
	manager.authorized = true
	manager.lastAttempt = time.Time{}
	manager.lastSuccess = manager.now().UTC()
	manager.lastError = ""
	manager.nextRetry = time.Time{}
	manager.mu.Unlock()
	manager.signal()
	return token, nil
}

func (manager *TokenManager) Refresh(ctx context.Context) (Token, error) {
	manager.operationMu.Lock()
	defer manager.operationMu.Unlock()

	manager.mu.RLock()
	authorized := manager.authorized
	refreshToken := manager.token.RefreshToken
	manager.mu.RUnlock()
	if !authorized || refreshToken == "" {
		return Token{}, ErrNotAuthorized
	}

	attemptedAt := manager.now().UTC()
	refreshed, err := manager.client.RefreshToken(ctx, refreshToken)
	if err != nil {
		manager.recordRefreshFailure(attemptedAt, err)
		return Token{}, err
	}
	if err := SaveSession(manager.sessionPath, refreshed); err != nil {
		manager.recordRefreshFailure(attemptedAt, err)
		return Token{}, err
	}

	manager.mu.Lock()
	manager.token = refreshed
	manager.authorized = true
	manager.lastAttempt = attemptedAt
	manager.lastSuccess = manager.now().UTC()
	manager.lastError = ""
	manager.nextRetry = time.Time{}
	manager.mu.Unlock()
	manager.signal()
	return refreshed, nil
}

func (manager *TokenManager) AccessToken(ctx context.Context) (string, error) {
	manager.mu.RLock()
	token := manager.token
	authorized := manager.authorized
	manager.mu.RUnlock()
	if !authorized || token.AccessToken == "" {
		return "", ErrNotAuthorized
	}
	now := manager.now().UTC()
	if token.AccessTokenExpiresAt.After(now.Add(manager.refreshBefore)) {
		return token.AccessToken, nil
	}
	refreshed, err := manager.Refresh(ctx)
	if err == nil {
		return refreshed.AccessToken, nil
	}
	if token.AccessTokenExpiresAt.After(now) {
		return token.AccessToken, nil
	}
	return "", fmt.Errorf("refresh expired XHS Spotlight access token: %w", err)
}

func (manager *TokenManager) ListCampaigns(ctx context.Context, request CampaignListRequest) (CampaignListData, error) {
	if _, err := normalizeCampaignListRequest(request); err != nil {
		return CampaignListData{}, err
	}
	accessToken, err := manager.AccessToken(ctx)
	if err != nil {
		return CampaignListData{}, err
	}
	return manager.client.ListCampaigns(ctx, accessToken, request)
}

func (manager *TokenManager) ListAllCampaigns(ctx context.Context, request CampaignListRequest) (CampaignCollection, error) {
	if _, err := normalizeCampaignListRequest(request); err != nil {
		return CampaignCollection{}, err
	}
	accessToken, err := manager.AccessToken(ctx)
	if err != nil {
		return CampaignCollection{}, err
	}
	return manager.client.ListAllCampaigns(ctx, accessToken, request)
}

func (manager *TokenManager) ListUnits(ctx context.Context, request UnitListRequest) (UnitListData, error) {
	if _, err := normalizeUnitListRequest(request); err != nil {
		return UnitListData{}, err
	}
	accessToken, err := manager.AccessToken(ctx)
	if err != nil {
		return UnitListData{}, err
	}
	return manager.client.ListUnits(ctx, accessToken, request)
}

func (manager *TokenManager) ListAllUnits(ctx context.Context, request UnitListRequest) (UnitCollection, error) {
	if _, err := normalizeUnitListRequest(request); err != nil {
		return UnitCollection{}, err
	}
	accessToken, err := manager.AccessToken(ctx)
	if err != nil {
		return UnitCollection{}, err
	}
	return manager.client.ListAllUnits(ctx, accessToken, request)
}

func (manager *TokenManager) ListCreativities(ctx context.Context, request CreativityListRequest) (CreativityListData, error) {
	if _, err := normalizeCreativityListRequest(request); err != nil {
		return CreativityListData{}, err
	}
	accessToken, err := manager.AccessToken(ctx)
	if err != nil {
		return CreativityListData{}, err
	}
	return manager.client.ListCreativities(ctx, accessToken, request)
}

func (manager *TokenManager) ListAllCreativities(ctx context.Context, request CreativityListRequest) (CreativityCollection, error) {
	if _, err := normalizeCreativityListRequest(request); err != nil {
		return CreativityCollection{}, err
	}
	accessToken, err := manager.AccessToken(ctx)
	if err != nil {
		return CreativityCollection{}, err
	}
	return manager.client.ListAllCreativities(ctx, accessToken, request)
}

func (manager *TokenManager) Status() ManagerStatus {
	manager.mu.RLock()
	token := manager.token
	status := ManagerStatus{
		Authorized:             manager.authorized,
		UserID:                 token.UserID,
		AppID:                  token.AppID,
		AdvertiserID:           token.AdvertiserID,
		ApprovalRoleType:       token.ApprovalRoleType,
		RoleType:               token.RoleType,
		PlatformType:           token.PlatformType,
		Scope:                  token.Scope,
		CorporationName:        token.CorporationName,
		VirtualSellerID:        token.VirtualSellerID,
		ApprovalAdvertisers:    append([]Advertiser(nil), token.ApprovalAdvertisers...),
		AccessTokenExpiresAt:   token.AccessTokenExpiresAt,
		RefreshTokenExpiresAt:  token.RefreshTokenExpiresAt,
		LastRefreshAttemptAt:   manager.lastAttempt,
		LastRefreshSucceededAt: manager.lastSuccess,
		LastRefreshError:       manager.lastError,
	}
	manager.mu.RUnlock()

	now := manager.now().UTC()
	status.AccessTokenValid = status.Authorized && status.AccessTokenExpiresAt.After(now)
	status.RefreshTokenValid = status.Authorized && status.RefreshTokenExpiresAt.After(now)
	return status
}

func (manager *TokenManager) Run(ctx context.Context) {
	for {
		delay, authorized := manager.nextRefreshDelay()
		if !authorized {
			select {
			case <-ctx.Done():
				return
			case <-manager.wake:
				continue
			}
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-manager.wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			continue
		case <-timer.C:
		}
		if _, err := manager.Refresh(ctx); err != nil && !errors.Is(err, context.Canceled) {
			if manager.onRefreshError != nil {
				manager.onRefreshError(err)
			}
		}
	}
}

func (manager *TokenManager) nextRefreshDelay() (time.Duration, bool) {
	manager.mu.RLock()
	authorized := manager.authorized
	expiresAt := manager.token.AccessTokenExpiresAt
	nextRetry := manager.nextRetry
	manager.mu.RUnlock()
	if !authorized {
		return 0, false
	}
	now := manager.now().UTC()
	if nextRetry.After(now) {
		return nextRetry.Sub(now), true
	}
	refreshAt := expiresAt.Add(-manager.refreshBefore)
	if !refreshAt.After(now) {
		return 0, true
	}
	return refreshAt.Sub(now), true
}

func (manager *TokenManager) recordRefreshFailure(attemptedAt time.Time, err error) {
	manager.mu.Lock()
	manager.lastAttempt = attemptedAt
	manager.lastError = err.Error()
	manager.nextRetry = attemptedAt.Add(manager.retryInterval)
	manager.mu.Unlock()
}

func (manager *TokenManager) signal() {
	select {
	case manager.wake <- struct{}{}:
	default:
	}
}
