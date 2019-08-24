package node

import (
	"path/filepath"
	"strings"

	"github.com/medyagh/kic/pkg/command"
	"github.com/medyagh/kic/pkg/oci"

	"github.com/pkg/errors"
)

// Node represents a handle to a kic node
// This struct must be created by one of: CreateControlPlane
type Node struct {
	// must be one of docker container ID or name
	name string
	// cached node info etc.
	cache *nodeCache
}

// WriteFile writes content to dest on the node
func (n *Node) WriteFile(localRunner command.Runner, dest, content string, perm string) error {
	// create destination directory
	args := []string{
		"mkdir", "-p", filepath.Dir(dest),
	}
	cmd := strings.Join(args, " ")
	out, err := localRunner.CombinedOutput(cmd)
	if err != nil {
		return errors.Wrapf(err, "WriteFile: failed to create directory %q. cmd: %s output: %s", dest, cmd, out)
	}

	args = []string{
		"cp", "/dev/stdin", dest,
	}

	cmd = strings.Join(args, " ")
	out, err = localRunner.CombinedOutput(cmd, strings.NewReader(content))
	if err != nil {
		return errors.Wrapf(err, "WriteFile: failed to run: cmd: %s , output: %s", cmd, out)
	}

	args = []string{
		"chmod", perm, dest,
	}

	cmd = strings.Join(args, " ")
	out, err = localRunner.CombinedOutput(cmd, strings.NewReader(content))
	if err != nil {
		return errors.Wrapf(err, "WriteFile: failed to run: %s %s , output: %s", cmd, dest, out)
	}

	return errors.Wrapf(err, "WriteFile: failed to run: chmod %s %s", perm, dest)
}

// IP returns the IP address of the node
func (n *Node) IP(localRunner command.Runner) (ipv4 string, ipv6 string, err error) {
	// use the cached version first
	cachedIPv4, cachedIPv6 := n.cache.IP()
	if cachedIPv4 != "" && cachedIPv6 != "" {
		return cachedIPv4, cachedIPv6, nil
	}
	// retrieve the IP address of the node using docker inspect
	lines, err := oci.Inspect(localRunner, n.name, "{{range .NetworkSettings.Networks}}{{.IPAddress}},{{.GlobalIPv6Address}}{{end}}")
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
