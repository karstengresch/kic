package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/medyagh/kic/example/single_node/myrunner"
	"github.com/medyagh/kic/pkg/config/cri"
	"github.com/medyagh/kic/pkg/image"
	"github.com/medyagh/kic/pkg/kube"
	"github.com/medyagh/kic/pkg/node"
	"github.com/medyagh/kic/pkg/oci"
	"github.com/phayes/freeport"
	"k8s.io/klog"
)

func main() {
	profile := flag.String("profile", "p1", "profile name")
	delete := flag.Bool("delete", false, "to delete")
	start := flag.Bool("start", false, "to start")
	hostIP := flag.String("host-ip", "127.0.0.1", "node's ip")
	kubeVersion := flag.String("kubernetes-version", "v1.15.0", "kuberentes version")
	flag.Parse()

	// Gets the base image to use for the nodes
	imgSha, _ := image.NameForVersion(*kubeVersion)

	hostPort := freePort()

	id := *profile + "control-plane"
	ns := &node.Spec{
		Profile:           *profile,
		Name:              id,
		Image:             imgSha,
		Role:              "control-plane",
		ExtraMounts:       []cri.Mount{},
		ExtraPortMappings: []cri.PortMapping{},
		APIServerAddress:  *hostIP,
		APIServerPort:     hostPort,
		IPv6:              false,
	}
	localRunner := myrunner.NewLocalRunner("docker")
	nodeRunner := myrunner.NewNodeRunner(id)

	if *start {
		fmt.Printf("Starting on port %d\n ", hostPort)
		err := oci.PullIfNotPresent(imgSha, false, time.Minute*3)
		if err != nil {
			klog.Errorf("Error pulling image %s", imgSha)
		}

		node, err := ns.Create(localRunner)
		if err != nil {
			klog.Fatalf("Fail: to create node spec: %v", err)
		}

		ip, _, err := node.IP(localRunner)
		if err != nil {
			klog.Fatalf("Fail: to get ip: %v ", err)
		}

		cfg := kube.ConfigData{
			ClusterName:          *profile,
			KubernetesVersion:    *kubeVersion,
			ControlPlaneEndpoint: ip + ":6443",
			APIBindPort:          6443,
			APIServerAddress:     *hostIP,
			Token:                "abcdef.0123456789abcdef",
			PodSubnet:            "10.244.0.0/16",
			ServiceSubnet:        "10.96.0.0/12",
			ControlPlane:         true,
			IPv6:                 false,
		}

		kCfg, err := kube.KubeAdmCfg(cfg)
		if err != nil {
			klog.Fatalf("Fail: to generate kube adm config content: %v ", err)
		}

		kaCfgPath := "/kic/kubeadm.conf"

		// copy the config to the node
		if err := node.WriteFile(nodeRunner, kaCfgPath, kCfg, "644"); err != nil {
			klog.Errorf("failed to copy kubeadm config to node : %v", err)
		}

		kube.RunKubeadmInit(nodeRunner, kaCfgPath, *hostIP, hostPort, *profile)
		kube.RunTaint(nodeRunner)
		kube.InstallOverlayNetwork(nodeRunner, "10.244.0.0/16")
		c, _ := kube.GenerateKubeConfig(nodeRunner, *hostIP, hostPort, *profile) // generates from the /etc/ inside container
		// kubeconfig for end-user
		kube.WriteKubeConfig(c, *profile)

	}

	if *delete {
		fmt.Printf("Deleting ... %s\n", *profile)
		ns.Delete(localRunner)

	}

}

// returns a free port on the system
func freePort() int32 {
	p, err := freeport.GetFreePort()
	hostPort := int32(p)
	if err != nil {
		klog.Fatal(err)
	}
	return hostPort
}
