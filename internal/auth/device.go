// Device flow (OAuth) for GitHub, so gpk needs no gh CLI and no
// manually created PAT. Uses GitHub CLI's public OAuth app client_id,
// the standard practice for third-party gh-free tools.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	deviceCodeURL = "https://github.com/login/device/code"
	tokenURL      = "https://github.com/login/oauth/access_token"

	// GitHub CLI's public OAuth app client_id.
	ClientID = "178c6fc778ccc68e1d6a"

	// Scopes needed for Projects v2 read AND write (moving items between
	// columns uses updateProjectV2ItemFieldValue, which needs 'project'),
	// plus repo metadata.
	Scopes = "repo read:project project"
)

// DeviceCode is the pending authorization returned by step 1.
type DeviceCode struct {
	UserCode        string // "XXXX-XXXX", entered by the user in the browser
	DeviceCode      string // internal, sent when polling
	VerificationURL string // https://github.com/login/device
	Interval        time.Duration
	ExpiresIn       time.Duration
}

type deviceCodeResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Error           string `json:"error"`
	ErrorDesc       string `json:"error_description"`
}

// RequestDeviceCode starts the device flow.
func RequestDeviceCode(ctx context.Context, hc *http.Client) (DeviceCode, error) {
	form := url.Values{
		"client_id": {ClientID},
		"scope":     {Scopes},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return DeviceCode{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := hc.Do(req)
	if err != nil {
		return DeviceCode{}, fmt.Errorf("request device code: %w", err)
	}
	defer resp.Body.Close()

	var dc deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&dc); err != nil {
		return DeviceCode{}, fmt.Errorf("decode device code response: %w", err)
	}
	if dc.Error != "" {
		return DeviceCode{}, fmt.Errorf("device code request failed: %s (%s)", dc.Error, dc.ErrorDesc)
	}
	if dc.DeviceCode == "" || dc.UserCode == "" {
		return DeviceCode{}, errors.New("device code response missing fields")
	}

	interval := dc.Interval
	if interval <= 0 {
		interval = 5
	}
	expires := dc.ExpiresIn
	if expires <= 0 {
		expires = 900
	}
	return DeviceCode{
		UserCode:        FormatUserCode(dc.UserCode),
		DeviceCode:      dc.DeviceCode,
		VerificationURL: dc.VerificationURI,
		Interval:        time.Duration(interval) * time.Second,
		ExpiresIn:       time.Duration(expires) * time.Second,
	}, nil
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Error       string `json:"error"`
	ErrorDesc   string `json:"error_description"`
}

// PollToken blocks until the user approves (or the code expires).
// progress, if non-nil, is called after each poll round.
func PollToken(ctx context.Context, hc *http.Client, dc DeviceCode, progress func(elapsed time.Duration)) (string, error) {
	deadline := time.Now().Add(dc.ExpiresIn)
	interval := dc.Interval

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(interval):
		}

		form := url.Values{
			"client_id":   {ClientID},
			"device_code": {dc.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := hc.Do(req)
		if err != nil {
			return "", fmt.Errorf("poll token: %w", err)
		}
		var tr tokenResponse
		err = json.NewDecoder(resp.Body).Decode(&tr)
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("decode token response: %w", err)
		}

		switch tr.Error {
		case "":
			if tr.AccessToken == "" {
				return "", errors.New("token response missing access_token")
			}
			return tr.AccessToken, nil
		case "authorization_pending":
			// keep waiting
		case "slow_down":
			// RFC 8628: add 5 seconds to the polling interval.
			interval += 5 * time.Second
		case "expired_token", "authorization_expired":
			return "", errors.New("device code expired, try again")
		default:
			return "", fmt.Errorf("device flow failed: %s (%s)", tr.Error, tr.ErrorDesc)
		}

		if time.Now().After(deadline) {
			return "", errors.New("device code expired, try again")
		}
		if progress != nil {
			progress(0)
		}
	}
}

// FormatUserCode inserts the dash GitHub expects: "ABCD1234" -> "ABCD-1234".
func FormatUserCode(code string) string {
	if len(code) == 8 && !strings.Contains(code, "-") {
		return code[:4] + "-" + code[4:]
	}
	return code
}

// Hyperlink renders label as a clickable link to url using OSC 8.
// Terminals and multiplexers differ in which form they honor, so callers
// offer several variants: BEL vs ST terminator, with or without an id.
func Hyperlink(url, label, id string, useBEL bool) string {
	params := ""
	if id != "" {
		params = "id=" + id + ";"
	}
	term := "\x1b\\"
	if useBEL {
		term = "\x07"
	}
	return "\x1b]8;;" + params + url + term + label + "\x1b]8;;" + term
}
