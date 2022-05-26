package main

import (
	"github.com/spectrocloud/vcluster/cmd/vclusterctl/cmd"
	"github.com/spectrocloud/vcluster/pkg/upgrade"
	"os"

	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var version string = ""

func main() {
	upgrade.SetVersion(version)

	cmd.Execute()
	os.Exit(0)
}
