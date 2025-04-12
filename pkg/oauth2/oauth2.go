package oauth2

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	stdio "io"
	"net/http"
	"net/url"
	"time"

	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/pkg/glinstance"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
	"gitlab.com/gitlab-org/cli/pkg/utils"
)

const (
	scopes = "openid profile read_user write_repository api"
)

// DeviceAuthResponse represents the response from the device authorization endpoint.
type DeviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"` // Time in seconds until the codes expire
	Interval                int    `json:"interval"`   // Minimum polling interval in seconds
}

func oAuthClientID(cfg config.Config, hostname string) (string, error) {
	if glinstance.IsSelfHosted(hostname) {
		clientID, err := cfg.Get(hostname, "client_id")
		if err != nil {
			return "", err
		}

		if clientID == "" {
			return "", fmt.Errorf("set 'client_id' first with `glab config set client_id <client_id> -g -h %s`", hostname)
		}
		return clientID, nil
	}
	return glinstance.DefaultClientID(), nil
}

// https://docs.gitlab.com/api/oauth2/#device-authorization-grant-flow
func StartFlow(cfg config.Config, io *iostreams.IOStreams, hostname string) (string, error) {
	// Construct the authorization and token URLs for the target GitLab instance.
	authURL := fmt.Sprintf("https://%s/oauth/authorize_device", hostname)
	tokenURL := fmt.Sprintf("https://%s/oauth/token", hostname)

	// Get the appropriate OAuth Client ID for the instance.
	clientID, err := oAuthClientID(cfg, hostname)
	if err != nil {
		return "", err // Return error from oAuthClientID (e.g., self-hosted ID not set)
	}

	// Step 1: Request device and user codes from the authorization server.
	deviceAuthResponse, err := requestDeviceAuthorization(authURL, clientID, scopes)
	if err != nil {
		return "", fmt.Errorf("failed to request device authorization: %w", err)
	}

	// Display instructions to the user.
	fmt.Fprintf(io.StdErr, "! First copy your one-time user code: %s%s%s\n", io.Color().Bold(deviceAuthResponse.UserCode), io.Color().RedCheck(), "")
	fmt.Fprintf(io.StdErr, "- Press Enter to open %s in your browser...", hostname)
	fmt.Scanln() // Wait for user confirmation before opening browser

	// Step 2: Open the verification URI (preferably the complete one) in the user's browser.
	verificationLink := deviceAuthResponse.VerificationURIComplete
	if verificationLink == "" { // Fallback if complete URI is not provided
		verificationLink = deviceAuthResponse.VerificationURI
		fmt.Fprintf(io.StdErr, "Navigate to: %s\n", deviceAuthResponse.VerificationURI)
		fmt.Fprintf(io.StdErr, "And enter code: %s\n", deviceAuthResponse.UserCode)
	}

	browser, _ := cfg.Get(hostname, "browser") // Get preferred browser from config, ignore error
	if err := utils.OpenInBrowser(verificationLink, browser); err != nil {
		fmt.Fprintf(io.StdErr, "%s Failed opening browser %s\n", io.Color().WarnIcon(), io.Color().Bold(verificationLink))
		fmt.Fprintf(io.StdErr, "Error: %s\n", err)
		fmt.Fprintf(io.StdErr, "Please open the URL manually in your browser and paste the user code shown above.\n")
	}

	fmt.Fprintf(io.StdErr, "Waiting for authorization...\n")

	// Step 3: Poll the token endpoint until the user authorizes or denies, or it times out.
	tokenResponse, err := pollForToken(io, tokenURL, clientID, deviceAuthResponse.DeviceCode, deviceAuthResponse.Interval, deviceAuthResponse.ExpiresIn)
	if err != nil {
		return "", fmt.Errorf("failed while polling for token: %w", err)
	}

	// Check if the token response contains an OAuth error.
	if tokenResponse.Error != "" {
		// Provide more specific feedback for common errors
		switch tokenResponse.Error {
		case "access_denied":
			return "", fmt.Errorf("authorization denied: %s", tokenResponse.ErrorDescription)
		case "expired_token":
			return "", fmt.Errorf("authorization timed out: The verification code expired. Please try again")
		default:
			return "", fmt.Errorf("token exchange failed: %s - %s", tokenResponse.Error, tokenResponse.ErrorDescription)
		}
	}

	// Ensure we received an access token
	if tokenResponse.AccessToken == "" {
		return "", fmt.Errorf("authentication failed: received empty access token")
	}

	fmt.Fprintf(io.StdErr, "%s Authorization successful.\n", io.Color().GreenCheck())

	// Assuming AuthToken struct definition NOW INCLUDES TokenType and Scope
	token := &AuthToken{
		AccessToken:  tokenResponse.AccessToken,
		ExpiresIn:    tokenResponse.ExpiresIn, // Pass ExpiresIn for calculation
		RefreshToken: tokenResponse.RefreshToken,
	}

	// Calculate the absolute expiration time.
	token.CalcExpiresDate() // This method should exist on the AuthToken struct

	// Persist the obtained token information in the configuration.
	err = token.SetConfig(hostname, cfg) // This method should exist on the AuthToken struct
	if err != nil {
		return "", fmt.Errorf("failed to save token configuration: %w", err)
	}

	// Return the access token for immediate use if needed.
	return token.AccessToken, nil
}

// requestDeviceAuthorization sends the initial request to the device authorization endpoint.
func requestDeviceAuthorization(authURL, clientID, scope string) (*DeviceAuthResponse, error) {
	// Prepare the form data for the POST request.
	form := url.Values{
		"client_id": []string{clientID},
		"scope":     []string{scope},
	}

	// Perform the POST request.
	resp, err := http.PostForm(authURL, form)
	if err != nil {
		return nil, fmt.Errorf("HTTP POST request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read the response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for non-success HTTP status codes.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Unmarshal the JSON response body into the DeviceAuthResponse struct.
	var deviceAuthResponse DeviceAuthResponse
	err = json.Unmarshal(body, &deviceAuthResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %w\nBody: %s", err, string(body))
	}

	// Basic validation of the response
	if deviceAuthResponse.DeviceCode == "" || deviceAuthResponse.UserCode == "" || deviceAuthResponse.VerificationURI == "" {
		return nil, fmt.Errorf("invalid response received: missing required fields\nBody: %s", string(body))
	}
	if deviceAuthResponse.Interval == 0 {
		deviceAuthResponse.Interval = 5 // Default interval if not provided by server
	}

	return &deviceAuthResponse, nil
}

// pollForToken periodically polls the token endpoint until authorization succeeds, fails, or times out.
func pollForToken(io *iostreams.IOStreams, tokenURL, clientID, deviceCode string, interval, expiresIn int) (*TokenResponse, error) {
	// Set a timeout for the entire polling process based on the expires_in value.
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(expiresIn)*time.Second)
	defer cancel()

	// Ensure a minimum polling interval.
	pollingInterval := time.Duration(interval) * time.Second
	minInterval := 5 * time.Second // Set a minimum sensible interval
	if pollingInterval < minInterval {
		pollingInterval = minInterval
	}

	// Start the polling loop.
	for {
		// Wait for the polling interval before making the request,
		// except for the very first attempt.
		select {
		case <-ctx.Done():
			// The timeout context expired before we got a definitive answer.
			return &TokenResponse{Error: "expired_token", ErrorDescription: "user did not authorize within the allowed time limit"}, nil // Simulate expired_token
		case <-time.After(pollingInterval):
			// Interval elapsed, proceed to poll.
		}

		// Prepare the form data for the token request.
		form := url.Values{
			"grant_type":  []string{"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": []string{deviceCode},
			"client_id":   []string{clientID},
		}

		// Perform the POST request to the token endpoint.
		resp, err := http.PostForm(tokenURL, form)
		if err != nil {
			// Network or connection error during polling, retry might be possible but often indicates a persistent issue.
			// Log the error and continue polling until timeout? Or return error immediately?
			// Returning error might be safer.
			fmt.Fprintf(io.StdErr, "Polling error: %s\n", err)
			// return nil, fmt.Errorf("token endpoint request failed: %w", err) // Option: Fail fast
			continue // Option: Log and retry
		}
		defer resp.Body.Close()

		// Read the response body.
		body, err := stdio.ReadAll(resp.Body)
		if err != nil {
			fmt.Fprintf(io.StdErr, "Polling error: failed to read response body: %s\n", err)
			// return nil, fmt.Errorf("failed to read token response body: %w", err) // Option: Fail fast
			continue // Option: Log and retry
		}

		// Unmarshal the JSON response body into the TokenResponse struct.
		var tokenResponse TokenResponse
		err = json.Unmarshal(body, &tokenResponse)
		if err != nil {
			fmt.Fprintf(io.StdErr, "Polling error: failed to decode JSON response: %s\nBody: %s\n", err, string(body))
			// return nil, fmt.Errorf("failed to decode token response JSON: %w", err) // Option: Fail fast
			continue // Option: Log and retry
		}

		// Process the token response based on the error field or success.
		if tokenResponse.Error == "" && tokenResponse.AccessToken != "" {
			// Success: Access token received.
			return &tokenResponse, nil
		}

		switch tokenResponse.Error {
		case "authorization_pending":
			// Expected response while user hasn't finished: Continue polling.
			// Optionally print a dot or message to show progress
			// fmt.Fprint(io.StdErr, ".")
		case "slow_down":
			// Server requested slower polling: Increase interval and continue.
			fmt.Fprintf(io.StdErr, "Server requested slowdown, increasing polling interval.\n")
			pollingInterval += time.Duration(interval) * time.Second // Simple increase, could use server hint if provided
		case "access_denied", "expired_token":
			// Terminal errors: Stop polling and return the response.
			return &tokenResponse, nil
		default:
			// Unexpected error from the token endpoint: Stop polling and return.
			fmt.Fprintf(io.StdErr, "Unexpected error during token polling: %s - %s\n", tokenResponse.Error, tokenResponse.ErrorDescription)
			return &tokenResponse, nil // Return the error response
		}

		// If we loop back, check context again before sleeping (handled by select at the start).
	}
}

func RefreshToken(hostname string, cfg config.Config, protocol string) error {
	token, err := tokenFromConfig(hostname, cfg)
	if err != nil {
		return err
	}

	// Check if token has expired
	if token.ExpiryDate.After(time.Now()) {
		return nil
	}

	clientID, err := oAuthClientID(cfg, hostname)
	if err != nil {
		return err
	}

	form := url.Values{
		"client_id":     []string{clientID},
		"grant_type":    []string{"refresh_token"},
		"refresh_token": []string{token.RefreshToken},
	}

	tokenURL := fmt.Sprintf("%s://%s/oauth/token", protocol, hostname)
	resp, err := http.PostForm(tokenURL, form)
	if err != nil {
		return err
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	at := AuthToken{}

	err = json.Unmarshal(respBytes, &at)
	if err != nil {
		return err
	}

	at.CalcExpiresDate()

	err = at.SetConfig(hostname, cfg)
	if err != nil {
		return err
	}

	err = cfg.Write()
	if err != nil {
		return err
	}

	return nil
}
