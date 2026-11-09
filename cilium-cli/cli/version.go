// SPDX-License-Identifier: Apache-2.0
// Copyright Authors of Cilium

package cli

import (
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/cilium/cilium/cilium-cli/defaults"
	"github.com/cilium/cilium/cilium-cli/internal/helm"
	"github.com/cilium/cilium/pkg/safeio"
)

const skipUpdateCheckEnvVar = "CILIUM_SKIP_UPDATE_CHECK"

// getLatestStableVersion reads the latest stable Cilium version from url,
// bounded by timeout: the default transport waits 30 s to dial a host that
// drops packets and has no bound on a connection that never answers, which
// hung "cilium version" on hosts without a route to GitHub.
func getLatestStableVersion(url string, timeout time.Duration) (string, error) {
	client := http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// The body of any other response, a 404 page for one, would be read as the version.
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	b, err := safeio.ReadAllLimit(resp.Body, safeio.KB)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(b)), nil
}

func newCmdVersion() *cobra.Command {
	var clientOnly, skipUpdateCheck bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Display detailed version information",
		Long:  `Displays information about the version of this software.`,
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Printf("cilium-cli: %s compiled with %v on %v/%v\n", defaults.CLIVersion, runtime.Version(), runtime.GOOS, runtime.GOARCH)
			fmt.Printf("cilium image (default): %s\n", helm.GetDefaultVersionString())
			// --client answers from the binary alone: no server, and no network either,
			// which CI and the packagers' sandboxes that run it rely on.
			if clientOnly {
				return nil
			}
			if !skipUpdateCheck {
				stable, err := getLatestStableVersion(defaults.StableVersionURL, defaults.StableVersionTimeout)
				if err != nil {
					fmt.Printf("cilium image (stable): unknown. Unable to obtain stable version. Reason: %s\n", err.Error())
				} else {
					fmt.Printf("cilium image (stable): %s\n", stable)
				}
			}
			version, err := RootK8sClient.GetRunningCiliumVersion(RootParams.HelmReleaseName)
			if err != nil {
				fmt.Printf("cilium image (running): unknown. Unable to obtain cilium version. Reason: %s\n", err.Error())
			} else {
				fmt.Printf("cilium image (running): %s\n", version)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&clientOnly, "client", false, "If true, shows client version only (no server or network access required)")
	// A site without a route to GitHub sets the variable once instead of the flag on every call,
	// as CILIUM_NAMESPACE does for --namespace.
	skipUpdateCheckDefault, _ := strconv.ParseBool(os.Getenv(skipUpdateCheckEnvVar))
	cmd.Flags().BoolVar(&skipUpdateCheck, "skip-update-check", skipUpdateCheckDefault, "If true, does not check GitHub for the latest stable Cilium release. Can also be set via CILIUM_SKIP_UPDATE_CHECK env var")
	return cmd
}
