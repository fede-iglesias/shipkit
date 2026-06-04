package adapters

import "testing"

// TestNewGitHubHTTP verifies that the constructor returns a non-nil adapter
// with production defaults set.
func TestNewGitHubHTTP(t *testing.T) {
	a := NewGitHubHTTP()
	if a == nil {
		t.Fatal("NewGitHubHTTP returned nil")
	}
	if a.BaseURL != "https://api.github.com" {
		t.Errorf("BaseURL = %q; want https://api.github.com", a.BaseURL)
	}
	if a.UserAgent != "shipkit-update/1.0" {
		t.Errorf("UserAgent = %q; want shipkit-update/1.0", a.UserAgent)
	}
	if a.Client == nil {
		t.Error("Client is nil; want http.DefaultClient")
	}
}
