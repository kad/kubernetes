/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/golang/glog"
	netutil "k8s.io/apimachinery/pkg/util/net"
	versionutil "k8s.io/kubernetes/pkg/util/version"
	pkgversion "k8s.io/kubernetes/pkg/version"
)

const (
	getReleaseVersionTimeout = time.Duration(10 * time.Second)
)

var (
	kubeReleaseBucketURL  = "https://dl.k8s.io"
	kubeReleaseRegex      = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-0-9a-zA-Z_\.+]*)?$`)
	kubeReleaseLabelRegex = regexp.MustCompile(`^[[:lower:]]+(-[-\w_\.]+)?$`)
	kubeBucketPrefixes    = regexp.MustCompile(`^((release|ci|ci-cross)/)?([-\w_\.+]+)$`)
)

type status404Error struct {
	message string
}

func (err status404Error) Error() string {
	return err.message
}

func isStatus404Error(err error) bool {
	if err == nil {
		return false
	}
	switch err.(type) {
	case status404Error:
		return true
	default:
		return false
	}
}

// KubernetesReleaseVersion is helper function that can fetch
// available version information from release servers based on
// label names, like "stable" or "latest".
//
// If argument is already semantic version string, it
// will return same string.
//
// In case of labels, it tries to fetch from release
// servers and then return actual semantic version.
//
// Available names on release servers:
//  stable      (latest stable release)
//  stable-1    (latest stable release in 1.x)
//  stable-1.0  (and similarly 1.1, 1.2, 1.3, ...)
//  latest      (latest release, including alpha/beta)
//  latest-1    (latest release in 1.x, including alpha/beta)
//  latest-1.0  (and similarly 1.1, 1.2, 1.3, ...)
func KubernetesReleaseVersion(version string) (string, error) {
	ver := normalizedBuildVersion(version)
	if len(ver) != 0 {
		return ver, nil
	}

	bucketURL, versionLabel, err := splitVersion(version)
	if err != nil {
		return "", err
	}

	// revalidate, if exact build from e.g. CI bucket requested.
	ver = normalizedBuildVersion(versionLabel)
	if len(ver) != 0 {
		return ver, nil
	}

	// kubeReleaseLabelRegex matches labels such as: latest, latest-1, latest-1.10
	if kubeReleaseLabelRegex.MatchString(versionLabel) {
		url := fmt.Sprintf("%s/%s.txt", bucketURL, versionLabel)
		body, err := fetchFromURL(url, getReleaseVersionTimeout)
		if err != nil {
			if !isStatus404Error(err) {
				return "", err
			}
			// Handle air-gapped environments by falling back to the client version.
			glog.Infof("could not fetch a Kubernetes version from the internet: %v", err)
			body, err = kubeadmVersion(pkgversion.Get().String())
			if err != nil {
				return "", err
			}
			glog.Infof("falling back to the local client version: %s", body)
		}
		// Re-validate received version and return.
		return KubernetesReleaseVersion(body)
	}
	return "", fmt.Errorf("version %q doesn't match patterns for neither semantic version nor labels (stable, latest, ...)", version)
}

// KubernetesVersionToImageTag is helper function that replaces all
// non-allowed symbols in tag strings with underscores.
// Image tag can only contain lowercase and uppercase letters, digits,
// underscores, periods and dashes.
// Current usage is for CI images where all of symbols except '+' are valid,
// but function is for generic usage where input can't be always pre-validated.
func KubernetesVersionToImageTag(version string) string {
	allowed := regexp.MustCompile(`[^-a-zA-Z0-9_\.]`)
	return allowed.ReplaceAllString(version, "_")
}

// KubernetesIsCIVersion checks if user requested CI version
func KubernetesIsCIVersion(version string) bool {
	subs := kubeBucketPrefixes.FindAllStringSubmatch(version, 1)
	if len(subs) == 1 && len(subs[0]) == 4 && strings.HasPrefix(subs[0][2], "ci") {
		return true
	}
	return false
}

// Internal helper: returns normalized build version (with "v" prefix if needed)
// If input doesn't match known version pattern, returns empty string.
func normalizedBuildVersion(version string) string {
	if kubeReleaseRegex.MatchString(version) {
		if strings.HasPrefix(version, "v") {
			return version
		}
		return "v" + version
	}
	return ""
}

// Internal helper: split version parts,
// Return base URL and cleaned-up version
func splitVersion(version string) (string, string, error) {
	var urlSuffix string
	subs := kubeBucketPrefixes.FindAllStringSubmatch(version, 1)
	if len(subs) != 1 || len(subs[0]) != 4 {
		return "", "", fmt.Errorf("invalid version %q", version)
	}

	switch {
	case strings.HasPrefix(subs[0][2], "ci"):
		// Just use whichever the user specified
		urlSuffix = subs[0][2]
	default:
		urlSuffix = "release"
	}
	url := fmt.Sprintf("%s/%s", kubeReleaseBucketURL, urlSuffix)
	return url, subs[0][3], nil
}

// Internal helper: return content of URL
func fetchFromURL(url string, timeout time.Duration) (string, error) {
	glog.V(2).Infof("fetching Kubernetes version from URL: %s", url)
	client := &http.Client{Timeout: timeout, Transport: netutil.SetOldTransportDefaults(&http.Transport{})}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("unable to get URL %q: %s", url, err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg := fmt.Sprintf("unable to fetch file. URL: %q, status: %v", url, resp.Status)
		// do special handling for 404, as this means that the version file is missing on the server
		if resp.StatusCode == http.StatusNotFound {
			return "", status404Error{message: msg}
		}
		return "", errors.New(msg)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to read content of URL %q: %s", url, err.Error())
	}
	return strings.TrimSpace(string(body)), nil
}

// kubeadmVersion returns the version of the client without metadata.
func kubeadmVersion(info string) (string, error) {
	v, err := versionutil.ParseSemantic(info)
	if err != nil {
		return "", fmt.Errorf("kubeadm version error: %v", err)
	}
	// There is no utility in versionutil to get the version without the metadata,
	// so this needs some manual formatting.
	// Discard offsets after a release label and keep the labels down to e.g. `alpha.0` instead of
	// including the offset e.g. `alpha.0.206`. This is done to comply with GCR image tags.
	pre := v.PreRelease()
	if len(pre) > 0 {
		split := strings.Split(pre, ".")
		if len(split) > 2 {
			pre = split[0] + "." + split[1] // exclude the third element
		} else if len(split) < 2 {
			pre = split[0] + ".0" // append .0 to a partial label
		}
		pre = "-" + pre
	}
	vStr := fmt.Sprintf("v%d.%d.%d%s", v.Major(), v.Minor(), v.Patch(), pre)
	return vStr, nil
}
