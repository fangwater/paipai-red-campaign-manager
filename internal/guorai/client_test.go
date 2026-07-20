package guorai

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEncryptPasswordRoundTrip(t *testing.T) {
	encrypted, err := encryptPassword("secret-password")
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := hex.DecodeString(encrypted)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher([]byte("yk9JffHtEG9yX2cZe!YfONIfS^#Z!GZD"))
	if err != nil {
		t.Fatal(err)
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, []byte("RseNl&qkEY@^LB%$")).CryptBlocks(plain, ciphertext)
	padding := int(plain[len(plain)-1])
	plain = plain[:len(plain)-padding]
	if got := string(plain); got != "secret-password" {
		t.Fatalf("decrypted password = %q", got)
	}
}

func TestValidateDateRange(t *testing.T) {
	if err := validateDateRange("2026-01-01", "2026-03-31", "2026-04-01"); err != nil {
		t.Fatalf("90-day range rejected: %v", err)
	}
	if err := validateDateRange("2026-01-01", "2026-04-01", "2026-04-01"); err == nil {
		t.Fatal("expected 91-day range to fail")
	}
	if err := validateDateRange("2026-07-10", "2026-07-17", "2026-07-16"); err == nil {
		t.Fatal("expected date after statistics cutoff to fail")
	}
}

func TestLoginPersistsAndReusesSessionForQuery(t *testing.T) {
	var queryPayload map[string]any
	var planQueryPayload map[string]any
	var brandFunctionKey string
	var ruleFunctionKey string
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case "/api/auth/login":
			http.SetCookie(writer, &http.Cookie{Name: loginCookieName, Value: "login-token", Path: "/", Secure: true, HttpOnly: true})
			writeJSON(t, writer, map[string]any{"retcode": 0, "errmsg": "success", "content": nil})
		case "/api/auth/getAuthCode":
			if _, err := request.Cookie(loginCookieName); err != nil {
				t.Errorf("auth code request missing login cookie: %v", err)
			}
			writeJSON(t, writer, map[string]any{
				"retcode": 0, "errmsg": "success", "content": map[string]any{"url": serverURL(request) + "/generate"},
			})
		case "/generate":
			http.SetCookie(writer, &http.Cookie{Name: authCookieName, Value: "auth-token", Path: "/", Secure: true, HttpOnly: true})
			http.Redirect(writer, request, "/", http.StatusFound)
		case "/":
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte("{}"))
		case "/crm-platform/api/merchant/preInfo":
			requireAuthCookie(t, request)
			writeJSON(t, writer, map[string]any{
				"retcode": 0, "errmsg": "success",
				"content": map[string]any{"account": map[string]any{"enterpriseId": 801, "accountType": 1, "username": "tester"}},
			})
		case "/media-delivery-merchant/enterpriseBrand/queryPassEnterpriseBrand":
			requireAuthCookie(t, request)
			brandFunctionKey = request.Header.Get("functionkey")
			if request.URL.Query().Get("enterpriseId") != "801" {
				t.Errorf("enterpriseId query = %q", request.URL.Query().Get("enterpriseId"))
			}
			writeJSON(t, writer, map[string]any{
				"retcode": 0, "content": []map[string]any{{"xhsBrandId": "350209", "xhsBrandName": "MegaRed", "brandBindMerFlag": true}},
			})
		case "/dbapi-access/bigdata/shuliangattention/getRule":
			requireAuthCookie(t, request)
			ruleFunctionKey = request.Header.Get("functionkey")
			writeJSON(t, writer, map[string]any{
				"retcode": 0, "content": map[string]any{"endDate": "2026-07-16", "tradeDataPeriod": "30"},
			})
		case "/media-delivery-merchant/followList/getFollowNoteList":
			requireAuthCookie(t, request)
			if got := request.Header.Get("functionkey"); got != functionKey {
				t.Errorf("note functionkey = %q", got)
			}
			if err := json.NewDecoder(request.Body).Decode(&queryPayload); err != nil {
				t.Errorf("decode query payload: %v", err)
			}
			writeJSON(t, writer, map[string]any{
				"retcode": 0,
				"content": map[string]any{
					"totalCount": 1,
					"data":       []map[string]any{{"noteId": "note-1", "noteName": "Example"}},
				},
			})
		case "/media-delivery-merchant/followList/getFollowPlanList":
			requireAuthCookie(t, request)
			if got := request.Header.Get("functionkey"); got != "MyFollowPlansList" {
				t.Errorf("plan functionkey = %q", got)
			}
			if err := json.NewDecoder(request.Body).Decode(&planQueryPayload); err != nil {
				t.Errorf("decode plan query payload: %v", err)
			}
			writeJSON(t, writer, map[string]any{
				"retcode": 0,
				"content": map[string]any{
					"totalCount": 1,
					"data":       []map[string]any{{"planId": "plan-1", "planName": "Example plan"}},
				},
			})
		default:
			http.NotFound(writer, request)
		}
	}))
	defer server.Close()

	sessionPath := filepath.Join(t.TempDir(), ".guorai", "session.json")
	endpoints := Endpoints{LoginBase: server.URL, MainGateway: server.URL, MediaBase: server.URL}
	client, err := NewClient(sessionPath, WithHTTPClient(server.Client()), WithEndpoints(endpoints))
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Login(context.Background(), "tester", "password"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("session mode = %o, want 600", got)
	}

	reloaded, err := NewClient(sessionPath, WithHTTPClient(server.Client()), WithEndpoints(endpoints))
	if err != nil {
		t.Fatal(err)
	}
	result, err := reloaded.QueryNotes(context.Background(), NotesFilter{
		BeginDate: "2026-07-09", EndDate: "2026-07-16", PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 1 || len(result.Data) != 1 || result.Filter.BrandID != "350209" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if queryPayload["dateType"] != float64(1) || queryPayload["getDate"] != "2026-07-09" || queryPayload["endDate"] != "2026-07-16" {
		t.Fatalf("unexpected query payload: %#v", queryPayload)
	}
	if queryPayload["enterpriseId"] != float64(801) {
		t.Fatalf("enterpriseId payload = %#v", queryPayload["enterpriseId"])
	}

	planResult, err := reloaded.QueryNotes(context.Background(), NotesFilter{
		BusinessType: BusinessTypePlan, BeginDate: "2026-07-09", EndDate: "2026-07-16", PageSize: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if planResult.Total != 1 || len(planResult.Data) != 1 || planResult.Filter.BusinessType != BusinessTypePlan {
		t.Fatalf("unexpected plan result: %+v", planResult)
	}
	if planQueryPayload["isAd"] != float64(2) || planQueryPayload["dateType"] != float64(1) {
		t.Fatalf("unexpected plan query payload: %#v", planQueryPayload)
	}
	if brandFunctionKey != "MyFollowPlansList" || ruleFunctionKey != "MyFollowPlansList" {
		t.Fatalf("plan setup function keys: brand=%q rule=%q", brandFunctionKey, ruleFunctionKey)
	}
}

func requireAuthCookie(t *testing.T, request *http.Request) {
	t.Helper()
	cookie, err := request.Cookie(authCookieName)
	if err != nil || cookie.Value != "auth-token" {
		t.Fatalf("missing auth cookie: %v", err)
	}
	if request.Header.Get("systemmold") != "4" {
		t.Errorf("systemmold header = %q", request.Header.Get("systemmold"))
	}
}

func serverURL(request *http.Request) string {
	return "https://" + request.Host
}

func writeJSON(t *testing.T, writer http.ResponseWriter, value any) {
	t.Helper()
	if err := json.NewEncoder(writer).Encode(value); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeBusinessType(t *testing.T) {
	if got, err := normalizeBusinessType(""); err != nil || got != BusinessTypeNote {
		t.Fatalf("empty type = %q, %v", got, err)
	}
	if got, err := normalizeBusinessType(" PLAN "); err != nil || got != BusinessTypePlan {
		t.Fatalf("normalized type = %q, %v", got, err)
	}
	if _, err := normalizeBusinessType("campaign"); err == nil {
		t.Fatal("expected unsupported business type to fail")
	}
}

func TestSelectBrand(t *testing.T) {
	brands := []Brand{
		{XHSBrandID: "1", XHSBrandName: "Unbound"},
		{XHSBrandID: "2", XHSBrandName: "Bound", BrandBindMerFlag: true},
	}
	selected, err := selectBrand(brands, "")
	if err != nil {
		t.Fatal(err)
	}
	if selected.XHSBrandID != "2" {
		t.Fatalf("selected brand = %s", selected.XHSBrandID)
	}
	_, err = selectBrand(brands, "missing")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected missing brand error, got %v", err)
	}
}

func TestDefaultDateRange(t *testing.T) {
	end, err := time.Parse(time.DateOnly, "2026-07-16")
	if err != nil {
		t.Fatal(err)
	}
	if got := end.AddDate(0, 0, -6).Format(time.DateOnly); got != "2026-07-10" {
		t.Fatalf("default start = %s", got)
	}
}
