package node

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/medyagh/kic/pkg/command"
	"github.com/medyagh/kic/pkg/config/cri"
	"github.com/pkg/errors"
)

// Spec describes a node to create purely from the container aspect
// this does not inlude eg starting kubernetes (see actions for that)
type Spec struct {
	Name              string
	Profile           string
	Role              string
	Image             string
	ExtraMounts       []cri.Mount
	ExtraPortMappings []cri.PortMapping
	APIServerPort     int32
	APIServerAddress  string
	IPv6              bool
}

// TODO this should only return a host and let others make it control plane
func (d *Spec) Create(localRunner command.Runner) (node *Node, err error) {
	switch d.Role {
	case "control-plane":
		node, err := CreateControlPlaneNode(d.Name, d.Image, ClusterLabelKey+d.Profile, d.APIServerAddress, d.APIServerPort, d.ExtraMounts, d.ExtraPortMappings, localRunner)
		return node, errors.Wrap(err, "CreateControlPlaneNode:")
	default:
		return nil, fmt.Errorf("unknown node role: %s", d.Role)
	}
	return node, err
}

// TODO move spec to host
func (d *Spec) Stop(localRunner command.Runner) error {
	out, err := localRunner.CombinedOutput("docker pause " + d.Name)
	if err != nil {
		return errors.Wrapf(err, "stopping node, output: %s", out)
	}
	return nil
}

func (d *Spec) Delete(localRunner command.Runner) error {
	out, err := localRunner.CombinedOutput("docker rm -f -v " + d.Name)
	if err != nil {
		return errors.Wrapf(err, "deleting node, output: %s", out)
	}
	return nil
}

// ListNodes lists all the nodes (containers) created by kic on the system
func (d *Spec) ListNodes(localRunner command.Runner) ([]string, error) {

	args := []string{
		"ps",
		"-q",         // quiet output for parsing
		"-a",         // show stopped nodes
		"--no-trunc", // don't truncate
		// filter for nodes with the cluster label
		"--filter", "label=" + ClusterLabelKey + d.Profile,
		// format to include friendly name and the cluster name
		"--format", fmt.Sprintf(`{{.Names}}\t{{.Label "%s"}}`, ClusterLabelKey+d.Profile),
	}
	var buff bytes.Buffer

	err := localRunner.CombinedOutputTo(strings.Join(args, " "), &buff)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("failed to list containers for %s, output: %v", d.Profile, buff))
	}

	lines := []string{}

	scanner := bufio.NewScanner(&buff)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// TODO: write unit tests for this, make it is own function
	names := []string{}
	for _, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			return nil, errors.Errorf("invalid output when listing containers: %s", line)

		}
		ns := strings.Split(parts[0], ",")
		names = append(names, ns...)
	}
	return names, nil

}

// CreateControlPlaneNode creates a contol-plane node
// and gets ready for exposing the the API server
func CreateControlPlaneNode(name, image, clusterLabel, listenAddress string, port int32, mounts []cri.Mount, portMappings []cri.PortMapping, localRunner command.Runner) (node *Node, err error) {
	// add api server port mapping
	portMappingsWithAPIServer := append(portMappings, cri.PortMapping{
		ListenAddress: listenAddress,
		HostPort:      port,
		ContainerPort: 6443,
	})
	node, err = CreateNode(
		name, image, clusterLabel, "control-plane", mounts, portMappingsWithAPIServer, localRunner,
		// publish selected port for the API server
		"--expose", fmt.Sprintf("%d", port),
	)
	if err != nil {
		return node, err
	}

	// stores the port mapping into the node internal state
	node.cache.set(func(cache *nodeCache) {
		cache.ports = map[int32]int32{6443: port}
	})
	return node, nil
}

// FromName creates a node handle from the node' Name
func FromName(name string) *Node {
	return &Node{
		name:  name,
		cache: &nodeCache{},
	}
}
