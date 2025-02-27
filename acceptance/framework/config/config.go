// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hashicorp/go-version"
	"gopkg.in/yaml.v2"
)

// HelmChartPath is the path to the helm chart.
// Note: this will need to be changed if this file is moved.
const (
	HelmChartPath     = "../../../charts/consul"
	LicenseSecretName = "license"
	LicenseSecretKey  = "key"
)

// TestConfig holds configuration for the test suite.
type TestConfig struct {
	Kubeconfig    string
	KubeContext   string
	KubeNamespace string

	EnableMultiCluster     bool
	SecondaryKubeconfig    string
	SecondaryKubeContext   string
	SecondaryKubeNamespace string

	EnableEnterprise  bool
	EnterpriseLicense string

	EnableOpenshift bool

	EnablePodSecurityPolicies bool

	EnableCNI bool

	EnableTransparentProxy bool

	DisablePeering bool

	HelmChartVersion       string
	ConsulImage            string
	ConsulK8SImage         string
	ConsulDataplaneImage   string
	ConsulVersion          *version.Version
	ConsulDataplaneVersion *version.Version
	EnvoyImage             string
	ConsulCollectorImage   string

	HCPResourceID string

	VaultHelmChartVersion string
	VaultServerVersion    string

	NoCleanupOnFailure bool
	DebugDirectory     string

	UseAKS  bool
	UseEKS  bool
	UseGKE  bool
	UseKind bool

	helmChartPath string
}

// HelmValuesFromConfig returns a map of Helm values
// that includes any non-empty values from the TestConfig.
func (t *TestConfig) HelmValuesFromConfig() (map[string]string, error) {
	helmValues := map[string]string{}

	// Set the enterprise image first if enterprise tests are enabled.
	// It can be overwritten by the -consul-image flag later.
	if t.EnableEnterprise {
		entImage, err := t.entImage()
		if err != nil {
			return nil, err
		}
		setIfNotEmpty(helmValues, "global.image", entImage)

		if t.EnterpriseLicense != "" {
			setIfNotEmpty(helmValues, "global.enterpriseLicense.secretName", LicenseSecretName)
			setIfNotEmpty(helmValues, "global.enterpriseLicense.secretKey", LicenseSecretKey)
		}
	}

	if t.EnableOpenshift {
		setIfNotEmpty(helmValues, "global.openshift.enabled", "true")
	}

	if t.EnablePodSecurityPolicies {
		setIfNotEmpty(helmValues, "global.enablePodSecurityPolicies", "true")
	}

	if t.EnableCNI {
		setIfNotEmpty(helmValues, "connectInject.cni.enabled", "true")
		// GKE is currently the only cloud provider that uses a different CNI bin dir.
		if t.UseGKE {
			setIfNotEmpty(helmValues, "connectInject.cni.cniBinDir", "/home/kubernetes/bin")
		}
	}

	setIfNotEmpty(helmValues, "connectInject.transparentProxy.defaultEnabled", strconv.FormatBool(t.EnableTransparentProxy))

	setIfNotEmpty(helmValues, "global.image", t.ConsulImage)
	setIfNotEmpty(helmValues, "global.imageK8S", t.ConsulK8SImage)
	setIfNotEmpty(helmValues, "global.imageEnvoy", t.EnvoyImage)
	setIfNotEmpty(helmValues, "global.imageConsulDataplane", t.ConsulDataplaneImage)

	return helmValues, nil
}

type values struct {
	Global globalValues `yaml:"global"`
}

type globalValues struct {
	Image string `yaml:"image"`
}

// entImage parses out consul version from values.yaml
// and sets global.image to the consul enterprise image with that version.
func (t *TestConfig) entImage() (string, error) {
	if t.helmChartPath == "" {
		t.helmChartPath = HelmChartPath
	}

	// Unmarshal values.yaml to current global.image value.
	valuesContents, err := os.ReadFile(filepath.Join(t.helmChartPath, "values.yaml"))
	if err != nil {
		return "", err
	}

	var v values
	err = yaml.Unmarshal(valuesContents, &v)
	if err != nil {
		return "", err
	}

	// Check if the image contains digest instead of a tag.
	// If it does, we want to use that image instead rather than
	// trying to change the tag to an enterprise tag.
	if strings.Contains(v.Global.Image, "@sha256") {
		return v.Global.Image, nil
	}

	// Otherwise, assume that we have an image tag with a version in it.
	consulImageSplits := strings.Split(v.Global.Image, ":")
	if len(consulImageSplits) != 2 {
		return "", fmt.Errorf("could not determine consul version from global.image: %s", v.Global.Image)
	}
	consulImageVersion := consulImageSplits[1]

	var preRelease string
	// Handle versions like 1.9.0-rc1.
	if strings.Contains(consulImageVersion, "-") {
		split := strings.Split(consulImageVersion, "-")
		consulImageVersion = split[0]
		preRelease = fmt.Sprintf("-%s", split[1])
	}

	return fmt.Sprintf("hashicorp/consul-enterprise:%s%s-ent", consulImageVersion, preRelease), nil
}

// setIfNotEmpty sets key to val in map m if value is not empty.
func setIfNotEmpty(m map[string]string, key, val string) {
	if val != "" {
		m[key] = val
	}
}
