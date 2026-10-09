package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// tunnelCredentials returns the tunnel server's SSH password: the user's
// Google ID token, then an ID token for the service account with audience
// https://<host>/<email>, separated by a space.
func tunnelCredentials(ctx context.Context, c *Config, host string) (password, email string, err error) {
	credsPath := c.googleCredentials()
	data, err := os.ReadFile(credsPath)
	if err != nil {
		return "", "", fmt.Errorf("reading Google credentials (run `gcp-dev-cred --login`): %w", err)
	}
	type authorizedUser struct {
		Type         string `json:"type"`
		ClientID     string `json:"client_id"`
		ClientSecret string `json:"client_secret"`
		RefreshToken string `json:"refresh_token"`
	}
	var file struct {
		authorizedUser
		// impersonated_service_account files, as mounted into devenv pods,
		// wrap the user's credential and name the service account
		SourceCredentials authorizedUser `json:"source_credentials"`
		ImpersonationURL  string         `json:"service_account_impersonation_url"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return "", "", fmt.Errorf("%s: %w", credsPath, err)
	}
	creds := file.authorizedUser
	serviceAccount := c.ServiceAccount
	if file.Type == "impersonated_service_account" {
		creds = file.SourceCredentials
		if serviceAccount == "" {
			serviceAccount = impersonatedAccount(file.ImpersonationURL)
		}
	}
	if creds.Type != "authorized_user" {
		return "", "", fmt.Errorf("%s: expected an authorized_user or impersonated_service_account credential, got %q", credsPath, file.Type)
	}
	if serviceAccount == "" {
		serviceAccount = defaultServiceAccount
	}

	// a fresh token source refreshes immediately, so the ID token has its full lifetime
	conf := &oauth2.Config{ClientID: creds.ClientID, ClientSecret: creds.ClientSecret, Endpoint: google.Endpoint}
	token, err := conf.TokenSource(ctx, &oauth2.Token{RefreshToken: creds.RefreshToken}).Token()
	if err != nil {
		return "", "", fmt.Errorf("refreshing Google credentials (run `gcp-dev-cred --login`): %w", err)
	}
	userToken, _ := token.Extra("id_token").(string)
	if userToken == "" {
		return "", "", errors.New("Google did not return an ID token; the credential needs the userinfo.email scope")
	}
	email, err = tokenEmail(userToken)
	if err != nil {
		return "", "", err
	}

	saToken, err := serviceAccountIDToken(ctx, token, serviceAccount, "https://"+host+"/"+email)
	if err != nil {
		return "", "", err
	}
	return userToken + " " + saToken, email, nil
}

func serviceAccountIDToken(ctx context.Context, token *oauth2.Token, serviceAccount, audience string) (string, error) {
	body, _ := json.Marshal(map[string]any{"audience": audience, "includeEmail": true})
	url := "https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/" + serviceAccount + ":generateIdToken"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	token.SetAuthHeader(req)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("impersonating %s (are you in gcp-developers@ or gcp-testers@?): %s: %s", serviceAccount, resp.Status, respBody)
	}

	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", err
	}
	return out.Token, nil
}

// impersonatedAccount extracts the service account email from an
// iamcredentials .../serviceAccounts/<email>:generateAccessToken URL.
func impersonatedAccount(url string) string {
	_, account, _ := strings.Cut(url, "/serviceAccounts/")
	account, _, _ = strings.Cut(account, ":")
	return account
}

// tokenEmail reads the email claim without verifying the token; the tunnel
// server does the verification.
func tokenEmail(idToken string) (string, error) {
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return "", errors.New("malformed ID token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Email == "" {
		return "", errors.New("ID token has no email")
	}
	return strings.ToLower(claims.Email), nil
}
