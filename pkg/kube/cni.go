package kube

import (
	"bytes"
	"html/template"
	"strings"

	"github.com/medyagh/kic/pkg/command"
	"github.com/pkg/errors"
)

// InstallOverlayNetwork installs the overlay network needed to make it work
func InstallOverlayNetwork(nodeRunner command.Runner, subnet string) error {
	// read the manifest from the node
	var raw bytes.Buffer
	args := []string{
		"cat", "/kind/manifests/default-cni.yaml",
	}
	err := nodeRunner.CombinedOutputTo(strings.Join(args, " "), &raw)
	if err != nil {
		return errors.Wrap(err, "failed to read CNI manifest")
	}

	manifest := raw.String()

	if strings.Contains(manifest, "would you kindly template this file") {
		t, err := template.New("cni-manifest").Parse(manifest)
		if err != nil {
			return errors.Wrap(err, "failed to parse CNI manifest template")
		}
		var out bytes.Buffer
		err = t.Execute(&out, &struct {
			PodSubnet string
		}{
			PodSubnet: subnet,
		})
		if err != nil {
			return errors.Wrap(err, "failed to execute CNI manifest template")
		}
		manifest = out.String()
	}

	args = []string{
		"kubectl", "create", "--kubeconfig=/etc/kubernetes/admin.conf",
		"-f", "-",
	}

	out, err := nodeRunner.CombinedOutput(strings.Join(args, " "), strings.NewReader(manifest))
	if err != nil {
		return errors.Wrapf(err, "failed to apply overlay network, output %s", out)
	}
	return nil
}
