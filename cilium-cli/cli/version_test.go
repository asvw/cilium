// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package cli

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cilium/cilium/cilium-cli/defaults"
)

func TestGetLatestStableVersion(t *testing.T) {
	stable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintln(w, "v1.20.4")
	}))
	defer stable.Close()
	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Answer once the client has given up, so that the test does not wait the full second.
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
		}
		fmt.Fprintln(w, "v1.20.4")
	}))
	defer slow.Close()
	unreachable := httptest.NewServer(http.NotFoundHandler())
	unreachable.Close()

	tests := []struct {
		name    string
		url     string
		timeout time.Duration
		want    string
		wantErr bool
	}{
		{name: "stable version", url: stable.URL, timeout: time.Second, want: "v1.20.4"},
		{name: "not found", url: notFound.URL, timeout: time.Second, wantErr: true},
		{name: "timeout", url: slow.URL, timeout: 50 * time.Millisecond, wantErr: true},
		{name: "unreachable", url: unreachable.URL, timeout: time.Second, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getLatestStableVersion(tt.url, tt.timeout)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestVersionClientMakesNoRequest(t *testing.T) {
	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		fmt.Fprintln(w, "v1.20.4")
	}))
	defer srv.Close()
	orig := defaults.StableVersionURL
	defaults.StableVersionURL = srv.URL
	t.Cleanup(func() { defaults.StableVersionURL = orig })

	cmd := NewDefaultCiliumCommand()
	cmd.SetArgs([]string{"version", "--client"})
	require.NoError(t, cmd.Execute())
	assert.Zero(t, requests.Load())
}

func TestVersionSkipUpdateCheckEnvVar(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want bool
	}{
		{name: "empty", env: "", want: false},
		{name: "true", env: "true", want: true},
		{name: "one", env: "1", want: true},
		{name: "false", env: "false", want: false},
		{name: "not a bool", env: "yes", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(skipUpdateCheckEnvVar, tt.env)
			skip, err := newCmdVersion().Flags().GetBool("skip-update-check")
			require.NoError(t, err)
			assert.Equal(t, tt.want, skip)
		})
	}
}
