package guorai

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultLoginBase   = "https://login-guorai.enbrands.com"
	defaultMainGateway = "https://gateway-guorai.enbrands.com"
	defaultMediaBase   = "https://tb-gateway.enbrands.com"
	webOrigin          = "https://guorai.enbrands.com"
	loginCookieName    = "s_auth_slPlatform"
	authCookieName     = "Authorization_slPlatform"
	functionKey        = "MyFollowNotesList"
)

var ErrSessionExpired = errors.New("Guorai session is missing or expired; run guorai login again")

type Endpoints struct {
	LoginBase   string
	MainGateway string
	MediaBase   string
}

type Option func(*Client)

func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) { c.httpClient = client }
}

func WithEndpoints(endpoints Endpoints) Option {
	return func(c *Client) { c.endpoints = endpoints }
}

type Client struct {
	httpClient  *http.Client
	jar         http.CookieJar
	endpoints   Endpoints
	sessionPath string
	session     Session
}

type Session struct {
	Version      int       `json:"version"`
	Username     string    `json:"username"`
	EnterpriseID int64     `json:"enterprise_id"`
	LoginCookie  string    `json:"login_cookie,omitempty"`
	AuthCookie   string    `json:"auth_cookie"`
	SavedAt      time.Time `json:"saved_at"`
}

type Account struct {
	EnterpriseID int64  `json:"enterpriseId"`
	AccountType  int    `json:"accountType"`
	Username     string `json:"username"`
}

type envelope struct {
	Retcode int             `json:"retcode"`
	Errmsg  string          `json:"errmsg"`
	Message string          `json:"message"`
	Content json.RawMessage `json:"content"`
}

func NewClient(sessionPath string, options ...Option) (*Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	c := &Client{
		jar: jar,
		endpoints: Endpoints{
			LoginBase:   defaultLoginBase,
			MainGateway: defaultMainGateway,
			MediaBase:   defaultMediaBase,
		},
		sessionPath: sessionPath,
	}
	for _, option := range options {
		option(c)
	}
	if c.httpClient == nil {
		c.httpClient = &http.Client{Timeout: 2 * time.Minute}
	}
	c.httpClient.Jar = jar
	if err := c.loadSession(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) Login(ctx context.Context, username, password string) error {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return errors.New("username and password are required")
	}
	encrypted, err := encryptPassword(password)
	if err != nil {
		return err
	}

	login, err := c.request(ctx, http.MethodPost, c.endpoints.LoginBase+"/api/auth/login", map[string]any{
		"username": username, "password": encrypted, "system": "slPlatform", "systemMold": 4,
	}, "", false)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	if login.Retcode != 0 {
		switch login.Retcode {
		case 101312, 101313:
			return fmt.Errorf("login requires phone or WeChat verification (retcode %d)", login.Retcode)
		case 101316, 101317:
			return fmt.Errorf("login requires MFA verification (retcode %d)", login.Retcode)
		default:
			return apiError("login", login)
		}
	}

	auth, err := c.request(ctx, http.MethodPost, c.endpoints.LoginBase+"/api/auth/getAuthCode", map[string]any{
		"redirectUrl": "", "loginType": 2, "clientId": "", "systemMold": 4, "system": "slPlatform",
	}, "", false)
	if err != nil {
		return fmt.Errorf("authorize session: %w", err)
	}
	if auth.Retcode != 0 {
		return apiError("authorize session", auth)
	}
	var authContent struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(auth.Content, &authContent); err != nil || authContent.URL == "" {
		return errors.New("authorize session returned an empty URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authContent.URL, nil)
	if err != nil {
		return fmt.Errorf("create authorization redirect request: %w", err)
	}
	req.Header.Set("Referer", c.endpoints.LoginBase+"/")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("follow authorization redirect: %w", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("follow authorization redirect: HTTP %d", resp.StatusCode)
	}

	account, err := c.account(ctx)
	if err != nil {
		return fmt.Errorf("validate login: %w", err)
	}
	c.session = Session{
		Version: 1, Username: username, EnterpriseID: account.EnterpriseID, SavedAt: time.Now(),
	}
	c.captureCookies()
	if c.session.AuthCookie == "" {
		return errors.New("authorization cookie was not established")
	}
	return c.saveSession()
}

func (c *Client) ValidateSession(ctx context.Context) (Account, error) {
	account, err := c.account(ctx)
	if err != nil {
		return Account{}, err
	}
	if account.EnterpriseID == 0 {
		return Account{}, errors.New("authenticated account has no enterpriseId")
	}
	if c.session.EnterpriseID != account.EnterpriseID {
		c.session.EnterpriseID = account.EnterpriseID
		c.session.SavedAt = time.Now()
		if err := c.saveSession(); err != nil {
			return Account{}, err
		}
	}
	return account, nil
}

func (c *Client) account(ctx context.Context) (Account, error) {
	response, err := c.request(ctx, http.MethodGet, c.endpoints.MainGateway+"/crm-platform/api/merchant/preInfo", nil, "framework", true)
	if err != nil {
		return Account{}, err
	}
	if response.Retcode == 101309 {
		return Account{}, ErrSessionExpired
	}
	if response.Retcode != 0 {
		return Account{}, apiError("get account", response)
	}
	var content struct {
		Account Account `json:"account"`
	}
	if err := json.Unmarshal(response.Content, &content); err != nil {
		return Account{}, fmt.Errorf("decode account: %w", err)
	}
	return content.Account, nil
}

func (c *Client) request(ctx context.Context, method, endpoint string, body any, key string, apiRequest bool) (envelope, error) {
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return envelope{}, fmt.Errorf("encode request: %w", err)
		}
		reader = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return envelope{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	}
	if apiRequest {
		req.Header.Set("Origin", webOrigin)
		req.Header.Set("Referer", webOrigin+"/")
		req.Header.Set("systemmold", "4")
		if key != "" {
			req.Header.Set("functionkey", key)
		}
	} else {
		req.Header.Set("Origin", c.endpoints.LoginBase)
		req.Header.Set("Referer", c.endpoints.LoginBase+"/")
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return envelope{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return envelope{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var result envelope
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 16<<20))
	decoder.UseNumber()
	if err := decoder.Decode(&result); err != nil {
		return envelope{}, fmt.Errorf("decode response: %w", err)
	}
	return result, nil
}

func (c *Client) loadSession() error {
	if c.sessionPath == "" {
		return nil
	}
	data, err := os.ReadFile(c.sessionPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Guorai session: %w", err)
	}
	if err := json.Unmarshal(data, &c.session); err != nil {
		return fmt.Errorf("decode Guorai session: %w", err)
	}
	loginURL, err := url.Parse(c.endpoints.LoginBase + "/")
	if err != nil {
		return err
	}
	cookies := make([]*http.Cookie, 0, 2)
	if c.session.LoginCookie != "" {
		cookies = append(cookies, &http.Cookie{
			Name: loginCookieName, Value: c.session.LoginCookie, Path: "/", Secure: true, HttpOnly: true,
		})
	}
	if c.session.AuthCookie != "" {
		domain := ""
		if strings.HasSuffix(loginURL.Hostname(), ".enbrands.com") {
			domain = ".enbrands.com"
		}
		cookies = append(cookies, &http.Cookie{
			Name: authCookieName, Value: c.session.AuthCookie, Domain: domain, Path: "/", Secure: true, HttpOnly: true,
		})
	}
	c.jar.SetCookies(loginURL, cookies)
	return nil
}

func (c *Client) captureCookies() {
	loginURL, _ := url.Parse(c.endpoints.LoginBase + "/")
	for _, cookie := range c.jar.Cookies(loginURL) {
		switch cookie.Name {
		case loginCookieName:
			c.session.LoginCookie = cookie.Value
		case authCookieName:
			c.session.AuthCookie = cookie.Value
		}
	}
}

func (c *Client) saveSession() error {
	if c.sessionPath == "" {
		return errors.New("session path is empty")
	}
	dir := filepath.Dir(c.sessionPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("secure session directory: %w", err)
	}
	data, err := json.MarshalIndent(c.session, "", "  ")
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".session-*")
	if err != nil {
		return fmt.Errorf("create temporary session: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("secure temporary session: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("write session: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close session: %w", err)
	}
	if err := os.Rename(tmpName, c.sessionPath); err != nil {
		return fmt.Errorf("replace session: %w", err)
	}
	return os.Chmod(c.sessionPath, 0o600)
}

func encryptPassword(password string) (string, error) {
	key := []byte("yk9JffHtEG9yX2cZe!YfONIfS^#Z!GZD")
	iv := []byte("RseNl&qkEY@^LB%$")
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create password cipher: %w", err)
	}
	padding := aes.BlockSize - len([]byte(password))%aes.BlockSize
	plain := append([]byte(password), bytes.Repeat([]byte{byte(padding)}, padding)...)
	encrypted := make([]byte, len(plain))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, plain)
	return hex.EncodeToString(encrypted), nil
}

func apiError(action string, response envelope) error {
	if response.Retcode == 101309 {
		return ErrSessionExpired
	}
	message := response.Errmsg
	if message == "" {
		message = response.Message
	}
	if message == "" {
		message = "unknown error"
	}
	return fmt.Errorf("%s: retcode=%d message=%s", action, response.Retcode, message)
}
