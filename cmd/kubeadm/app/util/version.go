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
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
)

var (
	kubeReleaseBucketURL  = "https://dl.k8s.io"
	kubeReleaseRegex      = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-0-9a-zA-Z_\.+]*)?$`)
	kubeReleaseLabelRegex = regexp.MustCompile(`^[[:lower:]]+(-[-\w_\.]+)?$`)
	kubeBucketPrefixes    = regexp.MustCompile(`^((release|ci|ci-cross)/)?([-\w_\.+]+)$`)
)

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
//
// URL parameter allows to specify custom release servers.
// Empty URL parameter points to official Kubernetes downloads server
// https://dl.k8s.io/
func KubernetesReleaseVersion(version, url string) (string, error) {
	if kubeReleaseRegex.MatchString(version) {
		if strings.HasPrefix(version, "v") {
			return version, nil
		}
		return "v" + version, nil
	} else if kubeReleaseLabelRegex.MatchString(version) {
		if url == "" {
			url = kubeReleaseBucketURL + "/release"
		}
		vurl := fmt.Sprintf("%s/%s.txt", url, version)
		resp, err := http.Get(vurl)
		if err != nil {
			return "", fmt.Errorf("unable to get URL %q: %s", url, err.Error())
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("unable to fetch release information. URL: %q Status: %v", url, resp.Status)
		}
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("unable to read content of URL %q: %s", url, err.Error())
		}
		// Re-validate received version and return.
		return KubernetesReleaseVersion(strings.TrimSpace(string(body)), url)
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

// KubernetesValidateVersion is a helper function that can validate
// if user requested release version or version from CI build area
// and then return validated and/or resolved from keywords like "latest".
//
// Examples of supported versions:
// v1.7.1 -- released version.
// release/v1.7.1 -- same as above.
// stable-1.7 or release/stable-1.7 -- labeled official releases
// ci/v1.8.0-alpha.1.123+012345678 -- CI explicit build version
// ci/latest-1.8 or ci-cross/latest-1.8 -- CI labeled builds
//
// Returns: version, build is release (false) or CI (true), error.
func KubernetesValidateVersion(version string) (string, bool, error) {
	isCI := false

	subs := kubeBucketPrefixes.FindAllStringSubmatch(version, 1)
	if len(subs) != 1 {
		return "", isCI, fmt.Errorf("invalid version %q", version)
	}

	urlSuffix := "release"
	if strings.HasPrefix(subs[0][2], "ci") {
		isCI = true
		urlSuffix = subs[0][2]
	}
	url := fmt.Sprintf("%s/%s", kubeReleaseBucketURL, urlSuffix)
	ver, err := KubernetesReleaseVersion(subs[0][3], url)
	return ver, isCI, err
}
