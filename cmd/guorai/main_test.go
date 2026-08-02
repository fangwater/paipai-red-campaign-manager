package main

import (
	"context"
	"errors"
	"flag"
	"testing"
	"time"

	"paipai-red-campaign-manager/internal/guorai"
	"paipai-red-campaign-manager/internal/store"
)

func TestRegisterFilterFlagsUpdatesReturnedFilter(t *testing.T) {
	t.Setenv("GUORAI_MERCHANT_ID", "1101791")
	flags := flag.NewFlagSet("query", flag.ContinueOnError)
	filter := registerFilterFlags(flags)
	if err := flags.Parse([]string{
		"--type", "plan", "--from", "2026-07-01", "--to", "2026-07-08",
	}); err != nil {
		t.Fatal(err)
	}
	if filter.BusinessType != guorai.BusinessTypePlan {
		t.Fatalf("business type = %q", filter.BusinessType)
	}
	if filter.BeginDate != "2026-07-01" || filter.EndDate != "2026-07-08" {
		t.Fatalf("date range = %s through %s", filter.BeginDate, filter.EndDate)
	}
	if filter.MerchantID != "1101791" {
		t.Fatalf("merchant ID = %q", filter.MerchantID)
	}
}

func TestGuoraiWindowDays(t *testing.T) {
	if got := guoraiWindowDays(guorai.BusinessTypeNote, 14, 14, 0); got != 14 {
		t.Fatalf("note window days = %d", got)
	}
	if got := guoraiWindowDays(guorai.BusinessTypePlan, 14, 14, 0); got != 14 {
		t.Fatalf("plan window days = %d", got)
	}
	if got := guoraiWindowDays(guorai.BusinessTypeNote, 14, 14, 10); got != 10 {
		t.Fatalf("overridden note window days = %d", got)
	}
	if got := guoraiWindowDays(guorai.BusinessTypePlan, 14, 14, 10); got != 10 {
		t.Fatalf("overridden plan window days = %d", got)
	}
}

func TestGuoraiRollingWindows(t *testing.T) {
	asOf := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	windows := guoraiRollingWindows(asOf, 15, 7)
	if len(windows) != 15 {
		t.Fatalf("window count = %d", len(windows))
	}
	if got := windows[0].End.Format(time.DateOnly); got != "2026-07-06" {
		t.Fatalf("oldest snapshot = %s", got)
	}
	if got := windows[0].Start.Format(time.DateOnly); got != "2026-06-30" {
		t.Fatalf("oldest window start = %s", got)
	}
	if got := windows[14].Start.Format(time.DateOnly); got != "2026-07-14" {
		t.Fatalf("latest window start = %s", got)
	}
	if got := windows[14].End.Format(time.DateOnly); got != "2026-07-20" {
		t.Fatalf("latest snapshot = %s", got)
	}
}

func TestGuoraiFourteenDayRollingWindow(t *testing.T) {
	asOf := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	windows := guoraiRollingWindows(asOf, 1, 14)
	if len(windows) != 1 {
		t.Fatalf("window count = %d", len(windows))
	}
	if got := windows[0].Start.Format(time.DateOnly); got != "2026-07-07" {
		t.Fatalf("window start = %s", got)
	}
	if got := windows[0].End.Format(time.DateOnly); got != "2026-07-20" {
		t.Fatalf("window end = %s", got)
	}
}

type fakeGuoraiLoginClient struct {
	username string
	password string
	logins   int
	err      error
}

func (f *fakeGuoraiLoginClient) Login(_ context.Context, username, password string) error {
	f.logins++
	f.username = username
	f.password = password
	return f.err
}

type fakeGuoraiCredentialsLoader struct {
	credentials store.GuoraiCredentials
	loads       int
	err         error
}

func (f *fakeGuoraiCredentialsLoader) LoadGuoraiCredentials(context.Context) (store.GuoraiCredentials, error) {
	f.loads++
	return f.credentials, f.err
}

func TestGuoraiWithStoredLoginRenewsExpiredSessionAndRetries(t *testing.T) {
	client := &fakeGuoraiLoginClient{}
	loader := &fakeGuoraiCredentialsLoader{credentials: store.GuoraiCredentials{
		Username: "account",
		Password: "password",
	}}
	attempts := 0
	result, err := guoraiWithStoredLogin(context.Background(), client, loader, func() (string, error) {
		attempts++
		if attempts == 1 {
			return "", guorai.ErrSessionExpired
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" || attempts != 2 {
		t.Fatalf("result = %q, attempts = %d", result, attempts)
	}
	if loader.loads != 1 {
		t.Fatalf("credential loads = %d", loader.loads)
	}
	if client.logins != 1 || client.username != "account" || client.password != "password" {
		t.Fatalf("login = (%d, %q, %q)", client.logins, client.username, client.password)
	}
}

func TestGuoraiWithStoredLoginDoesNotRetryOtherErrors(t *testing.T) {
	client := &fakeGuoraiLoginClient{}
	loader := &fakeGuoraiCredentialsLoader{}
	wantErr := errors.New("upstream unavailable")
	attempts := 0
	_, err := guoraiWithStoredLogin(context.Background(), client, loader, func() (string, error) {
		attempts++
		return "", wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v", err)
	}
	if attempts != 1 || loader.loads != 0 || client.logins != 0 {
		t.Fatalf("attempts = %d, credential loads = %d, logins = %d", attempts, loader.loads, client.logins)
	}
}
