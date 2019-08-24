package kube

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/medyagh/kic/pkg/command"
)

/// RunKubeadmInit runs kubeadm init on a node
func RunKubeadmInit(nodeRunner command.Runner, kubeadmCfgPath string, hostIP string, hostPort int32, profile string) (string, error) { // run kubeadm
	args := []string{
		// init because this is the control plane node
		"kubeadm", "init",
		"--ignore-preflight-errors=all",
		// specify our generated config file
		"--config=" + kubeadmCfgPath,
		"--skip-token-print",
		// increase verbosity for debugging
		"--v=6",
	}

	out, err := nodeRunner.CombinedOutput(strings.Join(args, " "))
	if err != nil {
		return out, errors.Wrapf(err, "failed to init node with kubeadm")
	}
	return out, nil
}

// if we are only provisioning one node, remove the master taint
// https://kubernetes.io/docs/setup/independent/create-cluster-kubeadm/#master-isolation
func RunTaint(nodeRunner command.Runner) error {
	args := []string{
		"kubectl", // kubectl inside the host
		"--kubeconfig=/etc/kubernetes/admin.conf",
		"taint", "nodes", "--all", "node-role.kubernetes.io/master-",
	}
	out, err := nodeRunner.CombinedOutput(strings.Join(args, " "))
	if err != nil {
		return errors.Wrapf(err, "failed to remove master taint. output: %s", out)
	}
	return nil
}
