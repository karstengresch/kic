package node

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/medyagh/kic/pkg/config/cri"
	"github.com/medyagh/kic/pkg/oci"
	"github.com/medyagh/kic/pkg/runner"

	"github.com/pkg/errors"
)

const (
	// Docker default bridge network is named "bridge" (https://docs.docker.com/network/bridge/#use-the-default-bridge-network)
	DefaultNetwork  = "bridge"
	ClusterLabelKey = "io.k8s.sigs.kic.cluster" // ClusterLabelKey is applied to each node docker container for identification
	NodeRoleKey     = "io.k8s.sigs.kic.role"
)

// Node represents a handle to a kic node
// This struct must be created by one of: CreateControlPlane
type Node struct {
	// must be one of docker container ID or name
	name string
	// cached node info etc.
	cache *nodeCache
	cmder runner.Cmder
}

// WriteFile writes content to dest on the node
func (n *Node) WriteFile(dest, content string, perm string) error {
	// create destination directory
	cmd := n.Command("mkdir", "-p", filepath.Dir(dest))
	_, err := runner.RunLoggingOutputOnFail(cmd)
	if err != nil {
		return errors.Wrapf(err, "failed to create directory %s", dest)
	}

	err = n.Command("cp", "/dev/stdin", dest).SetStdin(strings.NewReader(content)).Run()
	if err != nil {
		return errors.Wrapf(err, "failed to run: cp /dev/stdin %s", dest)
	}
	err = n.Command("chmod", perm, dest).Run()
	return errors.Wrapf(err, "failed to run: chmod %s %s", perm, dest)
}

// IP returns the IP address of the node
func (n *Node) IP() (ipv4 string, ipv6 string, err error) {
	// use the cached version first
	cachedIPv4, cachedIPv6 := n.cache.IP()
	if cachedIPv4 != "" && cachedIPv6 != "" {
		return cachedIPv4, cachedIPv6, nil
	}
	// retrieve the IP address of the node using docker inspect
	lines, err := oci.Inspect(n.name, "{{range .NetworkSettings.Networks}}{{.IPAddress}},{{.GlobalIPv6Address}}{{end}}")
	if err != nil {
		return "", "", errors.Wrap(err, "failed to get container details")
	}
	if len(lines) != 1 {
		return "", "", errors.Errorf("file should only be one line, got %d lines", len(lines))
	}
	ips := strings.Split(lines[0], ",")
	if len(ips) != 2 {
		return "", "", errors.Errorf("container addresses should have 2 values, got %d values", len(ips))
	}
	n.cache.set(func(cache *nodeCache) {
		cache.ipv4 = ips[0]
		cache.ipv6 = ips[1]
	})
	return ips[0], ips[1], nil
}

// LoadImageArchive loads an image form archive into node
func (n *Node) LoadImageArchive(image io.Reader) error {
	cmd := n.Command(
		"ctr", "--namespace=k8s.io", "images", "import", "-",
	)
	cmd.SetStdin(image)
	if err := cmd.Run(); err != nil {
		return errors.Wrap(err, "failed to load image")
	}
	return nil
}

// Command returns a new runner.Cmd that will run on the node
func (n *Node) Command(command string, args ...string) runner.Cmd {
	return n.cmder.Command(command, args...)
}

// todo use a struct for this
func CreateNode(name, image, clusterLabel, role string, mounts []cri.Mount, portMappings []cri.PortMapping, cpus string, memory string, envs map[string]string, cmder runner.Cmder, extraArgs ...string) (*Node, error) {
	runArgs := []string{
		fmt.Sprintf("--cpus=%s", cpus),
		fmt.Sprintf("--memory=%s", memory),
		"-d", // run the container detached
		"-t", // allocate a tty for entrypoint logs
		// running containers in a container requires privileged
		// NOTE: we could try to replicate this with --cap-add, and use less
		// privileges, but this flag also changes some mounts that are necessary
		// including some ones docker would otherwise do by default.
		// for now this is what we want. in the future we may revisit this.
		"--privileged",
		"--security-opt", "seccomp=unconfined", // also ignore seccomp
		"--tmpfs", "/tmp", // various things depend on working /tmp
		"--tmpfs", "/run", // systemd wants a writable /run
		// some k8s things want /lib/modules
		"-v", "/lib/modules:/lib/modules:ro",
		"--hostname", name, // make hostname match container name
		"--name", name, // ... and set the container name
		// label the node with the cluster ID
		"--label", clusterLabel,
		// label the node with the role ID
		"--label", fmt.Sprintf("%s=%s", NodeRoleKey, role),
	}

	for key, val := range envs {
		runArgs = append(runArgs, "-e", fmt.Sprintf("%s=%s", key, val))
	}

	// adds node specific args
	runArgs = append(runArgs, extraArgs...)

	if oci.UsernsRemap() {
		// We need this argument in order to make this command work
		// in systems that have userns-remap enabled on the docker daemon
		runArgs = append(runArgs, "--userns=host")
	}

	_, err := oci.CreateContainer(
		image,
		oci.WithRunArgs(runArgs...),
		oci.WithMounts(mounts),
		oci.WithPortMappings(portMappings),
	)

	// we should return a handle so the caller can clean it up
	node := FromName(name)
	node.cmder = cmder
	if err != nil {
		return node, fmt.Errorf("docker run error %v", err)
	}

	return node, nil
}

// CreateControlPlaneNode creates a contol-plane node
// and gets ready for exposing the the API server
func CreateControlPlaneNode(name, image, clusterLabel, listenAddress string, port int32, mounts []cri.Mount, portMappings []cri.PortMapping, cpus string, memory string, envs map[string]string, cmder runner.Cmder) (node *Node, err error) {
	// add api server port mapping
	portMappingsWithAPIServer := append(portMappings, cri.PortMapping{
		ListenAddress: listenAddress,
		HostPort:      port,
		ContainerPort: 6443,
	})
	node, err = CreateNode(
		name, image, clusterLabel, "control-plane", mounts, portMappingsWithAPIServer, cpus, memory, envs, cmder,
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

// Find finds a node
func Find(name string, cmder runner.Cmder) (*Node, error) {
	// TODO: check node exists

	return &Node{
		name:  name,
		cache: &nodeCache{},
		cmder: cmder,
	}, nil
}
