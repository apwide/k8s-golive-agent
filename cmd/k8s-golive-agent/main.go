package main

import (
	"flag"
	"github.com/apwide/k8s-golive-agent/pkg/k8s"
	"github.com/apwide/k8s-golive-agent/pkg/signals"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	goliveconfig string
	kubeconfig   string
	master       string
)

func main() {
	flag.Parse()
	ctx := signals.SetupSignalHandler()

	kubeConfig, err := clientcmd.BuildConfigFromFlags(master, k8s.ExpandUserHome(kubeconfig))
	if err != nil {
		panic(err)
	}

	goliveConfig, err := k8s.LoadConfig(k8s.ExpandUserHome(goliveconfig))
	if err != nil {
		panic(err)
	}

	k8s.Start(ctx, kubeConfig, goliveConfig)
}

//var (
//	masterURL  string
//	kubeconfig string
//)

func init() {
	flag.StringVar(&goliveconfig, "goliveconfig", "/etc/golive/config.yaml", "Path to a golive config file. Only required if not default mounted path.")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&master, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	klog.InitFlags(nil)
}
