package action

import (
	"bytes"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/version"
)

// ConfigData is supplied to the kubeadm config template, with values populated
// by the cluster package
type ConfigData struct {
	ClusterName       string
	KubernetesVersion string
	// The ControlPlaneEndpoint, that is the address of the external loadbalancer
	// if defined or the bootstrap node
	ControlPlaneEndpoint string
	// The Local API Server port
	APIBindPort int
	// The API server external listen IP (which we will port forward)
	APIServerAddress string
	// ControlPlane flag specifies the node belongs to the control plane
	ControlPlane bool
	// The main IP address of the node
	NodeAddress string
	// The Token for TLS bootstrap
	Token string
	// The subnet used for pods
	PodSubnet string
	// The subnet used for services
	ServiceSubnet string
	// IPv4 values take precedence over IPv6 by default, if true set IPv6 default values
	IPv6 bool
	// DerivedConfigData is populated by Derive()
	// These auto-generated fields are available to Config templates,
	// but not meant to be set by hand
	DerivedConfigData
}

// DerivedConfigData fields are automatically derived by
// ConfigData.Derive if they are not specified / zero valued
type DerivedConfigData struct {
	// DockerStableTag is automatically derived from KubernetesVersion
	DockerStableTag string
}

// Derive automatically derives DockerStableTag if not specified
func (c *ConfigData) Derive() {
	if c.DockerStableTag == "" {
		c.DockerStableTag = strings.Replace(c.KubernetesVersion, "+", "_", -1)
	}
}

// templateExec executes the right kubeadm config template based on config data
func templateExec(data ConfigData) (config string, err error) {
	ver, err := version.ParseGeneric(data.KubernetesVersion)
	if err != nil {
		return "", err
	}

	// assume the latest API version, then fallback if the k8s version is too low
	templateSource := ConfigTemplateBetaV2
	if ver.LessThan(version.MustParseSemantic("v1.12.0")) {
		templateSource = ConfigTemplateAlphaV2
	} else if ver.LessThan(version.MustParseSemantic("v1.13.0")) {
		templateSource = ConfigTemplateAlphaV3
	} else if ver.LessThan(version.MustParseSemantic("v1.15.0")) {
		templateSource = ConfigTemplateBetaV1
	}

	t, err := template.New("kubeadm-config").Parse(templateSource)
	if err != nil {
		return "", errors.Wrap(err, "failed to parse config template")
	}

	// derive any automatic fields if not supplied
	data.Derive()

	// execute the template
	var buff bytes.Buffer
	err = t.Execute(&buff, data)
	if err != nil {
		return "", errors.Wrap(err, "error executing config template")
	}
	return buff.String(), nil
}
