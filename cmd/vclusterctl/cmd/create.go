package cmd

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spectrocloud/vcluster/cmd/vclusterctl/cmd/app/create"
	"github.com/spectrocloud/vcluster/pkg/helm/values"
	"github.com/spectrocloud/vcluster/pkg/upgrade"
	"github.com/spectrocloud/vcluster/pkg/util"
	"golang.org/x/mod/semver"

	"github.com/pkg/errors"
	"github.com/spectrocloud/vcluster/cmd/vclusterctl/flags"
	"github.com/spectrocloud/vcluster/cmd/vclusterctl/log"
	"github.com/spectrocloud/vcluster/pkg/helm"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	AllowedDistros = []string{"k3s", "k0s", "k8s", "eks"}
)

// CreateCmd holds the login cmd flags
type CreateCmd struct {
	*flags.GlobalFlags
	create.CreateOptions

	log log.Logger
}

// NewCreateCmd creates a new command
func NewCreateCmd(globalFlags *flags.GlobalFlags) *cobra.Command {
	cmd := &CreateCmd{
		GlobalFlags: globalFlags,
		log:         log.GetInstance(),
	}

	cobraCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new virtual cluster",
		Long: `
#######################################################
################### vcluster create ###################
#######################################################
Creates a new virtual cluster

Example:
vcluster create test --namespace test
#######################################################
	`,
		Args: cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			// Check for newer version
			upgrade.PrintNewerVersionWarning()
			validateDeprecated(&cmd.CreateOptions, cmd.log)
			return cmd.Run(args)
		},
	}

	cobraCmd.Flags().StringVar(&cmd.ChartVersion, "chart-version", upgrade.GetVersion(), "The virtual cluster chart version to use (e.g. v0.4.0)")
	cobraCmd.Flags().StringVar(&cmd.ChartName, "chart-name", "vcluster", "The virtual cluster chart name to use")
	cobraCmd.Flags().StringVar(&cmd.ChartRepo, "chart-repo", "https://charts.loft.sh", "The virtual cluster chart repo to use")
	cobraCmd.Flags().StringVar(&cmd.LocalChartDir, "local-chart-dir", "", "The virtual cluster local chart dir to use")
	cobraCmd.Flags().StringVar(&cmd.K3SImage, "k3s-image", "", "DEPRECATED: use --extra-values instead")
	cobraCmd.Flags().StringVar(&cmd.Distro, "distro", "k3s", fmt.Sprintf("Kubernetes distro to use for the virtual cluster. Allowed distros: %s", strings.Join(AllowedDistros, ", ")))
	cobraCmd.Flags().StringVar(&cmd.ReleaseValues, "release-values", "", "DEPRECATED: use --extra-values instead")
	cobraCmd.Flags().StringVar(&cmd.KubernetesVersion, "kubernetes-version", "", "The kubernetes version to use (e.g. v1.20). Patch versions are not supported")
	cobraCmd.Flags().StringSliceVarP(&cmd.ExtraValues, "extra-values", "f", []string{}, "Path where to load extra helm values from")
	cobraCmd.Flags().BoolVar(&cmd.CreateNamespace, "create-namespace", true, "If true the namespace will be created if it does not exist")
	cobraCmd.Flags().BoolVar(&cmd.DisableIngressSync, "disable-ingress-sync", false, "If true the virtual cluster will not sync any ingresses")
	cobraCmd.Flags().BoolVar(&cmd.CreateClusterRole, "create-cluster-role", false, "DEPRECATED: cluster role is now automatically created if it is required by one of the resource syncers that are enabled by the .sync.RESOURCE.enabled=true helm value, which is set in a file that is passed via --extra-values argument.")
	cobraCmd.Flags().BoolVar(&cmd.Expose, "expose", false, "If true will create a load balancer service to expose the vcluster endpoint")
	cobraCmd.Flags().BoolVar(&cmd.Connect, "connect", false, "If true will run vcluster connect directly after the vcluster was created")
	cobraCmd.Flags().BoolVar(&cmd.Upgrade, "upgrade", false, "If true will try to upgrade the vcluster instead of failing if it already exists")
	cobraCmd.Flags().BoolVar(&cmd.Isolate, "isolate", false, "If true vcluster and its workloads will run in an isolated environment")
	return cobraCmd
}

func validateDeprecated(createOptions *create.CreateOptions, log log.Logger) {
	if createOptions.ReleaseValues != "" {
		log.Warn("Flag --release-values is deprecated, please use --extra-values instead. This flag will be removed in future!")
	}
	if createOptions.K3SImage != "" {
		log.Warn("Flag --k3s-image is deprecated, please use --extra-values instead. This flag will be removed in future!")
	}
	if createOptions.CreateClusterRole {
		log.Warn("Flag --create-cluster-role is deprecated. Cluster role is now automatically created if it is required by one of the resource syncers that are enabled by the .sync.RESOURCE.enabled=true helm value, which is set in a file that is passed via --extra-values (or -f) argument.")
	}
}

// Run executes the functionality
func (cmd *CreateCmd) Run(args []string) error {
	// test for helm
	helmExecutablePath, err := exec.LookPath("helm")
	if err != nil {
		return fmt.Errorf("seems like helm is not installed. Helm is required for the creation of a virtual cluster. Please visit https://helm.sh/docs/intro/install/ for install instructions")
	}

	output, err := exec.Command(helmExecutablePath, "version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("seems like there are issues with your helm client: \n\n%s", output)
	}

	// first load the kube config
	kubeClientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(clientcmd.NewDefaultClientConfigLoadingRules(), &clientcmd.ConfigOverrides{
		CurrentContext: cmd.Context,
	})

	// load the raw config
	rawConfig, err := kubeClientConfig.RawConfig()
	if err != nil {
		return fmt.Errorf("there is an error loading your current kube config (%v), please make sure you have access to a kubernetes cluster and the command `kubectl get namespaces` is working", err)
	}
	if cmd.Context != "" {
		rawConfig.CurrentContext = cmd.Context
	}

	// load the rest config
	kubeConfig, err := kubeClientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("there is an error loading your current kube config (%v), please make sure you have access to a kubernetes cluster and the command `kubectl get namespaces` is working", err)
	}

	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return err
	}

	if cmd.Namespace == "" {
		cmd.Namespace, _, err = kubeClientConfig.Namespace()
		if err != nil {
			return err
		} else if cmd.Namespace == "" {
			cmd.Namespace = "default"
		}
	}

	// make sure namespace exists
	_, err = client.CoreV1().Namespaces().Get(context.Background(), cmd.Namespace, metav1.GetOptions{})
	if err != nil {
		if kerrors.IsNotFound(err) {
			// try to create the namespace
			cmd.log.Infof("Creating namespace %s", cmd.Namespace)
			_, err = client.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: cmd.Namespace,
				},
			}, metav1.CreateOptions{})
			if err != nil {
				return errors.Wrap(err, "create namespace")
			}
		} else if !kerrors.IsForbidden(err) {
			return err
		}
	}

	// get service cidr
	if cmd.CIDR == "" {
		cmd.CIDR = values.GetServiceCIDR(client, cmd.Namespace)
	}

	var kubernetesVersion *version.Info
	if cmd.KubernetesVersion != "" {
		if cmd.KubernetesVersion[0] != 'v' {
			cmd.KubernetesVersion = "v" + cmd.KubernetesVersion
		}

		if !semver.IsValid(cmd.KubernetesVersion) {
			return fmt.Errorf("please use valid semantic versioning format, e.g. vX.X")
		}

		majorMinorVer := semver.MajorMinor(cmd.KubernetesVersion)

		if splittedVersion := strings.Split(cmd.KubernetesVersion, "."); len(splittedVersion) > 2 {
			cmd.log.Warnf("currently we only support major.minor version (%s) and not the patch version (%s)", majorMinorVer, cmd.KubernetesVersion)
		}

		kubernetesVersion, err = values.ParseKubernetesVersionInfo(majorMinorVer)
		if err != nil {
			return err
		}
	}

	if kubernetesVersion == nil {
		kubernetesVersion, err = client.DiscoveryClient.ServerVersion()
		if err != nil {
			return err
		}
	}

	// load the default values
	chartOptions, err := cmd.ToChartOptions(kubernetesVersion)
	if err != nil {
		return err
	}
	chartValues, err := values.GetDefaultReleaseValues(chartOptions, cmd.log)
	if err != nil {
		return err
	}
	if cmd.ReleaseValues != "" {
		cmd.ExtraValues = append(cmd.ExtraValues, cmd.ReleaseValues)
	}

	// check if vcluster already exists
	if !cmd.Upgrade {
		release, err := helm.NewSecrets(client).Get(context.Background(), args[0], cmd.Namespace)
		if err != nil && !kerrors.IsNotFound(err) {
			return errors.Wrap(err, "get helm releases")
		} else if release != nil && release.Chart != nil && release.Chart.Metadata != nil && (release.Chart.Metadata.Name == "vcluster" || release.Chart.Metadata.Name == "vcluster-k0s" || release.Chart.Metadata.Name == "vcluster-k8s") {
			return fmt.Errorf("vcluster %s already exists in namespace %s. If you want to upgrade the existing vcluster release, run with the --upgrade flag", args[0], cmd.Namespace)
		}
	}

	// convert extra values
	extraValues := []string{}
	if len(cmd.ExtraValues) > 0 {
		for _, file := range cmd.ExtraValues {
			if strings.HasPrefix(file, "http://") || strings.HasPrefix(file, "https://") {
				extraValues = append(extraValues, file)
				continue
			}

			out, err := ioutil.ReadFile(file)
			if err != nil {
				return errors.Wrap(err, "read values file")
			} else if !strings.Contains(string(out), "##CIDR##") {
				extraValues = append(extraValues, file)
				continue
			}

			tempFile, err := ioutil.TempFile("", "")
			if err != nil {
				return errors.Wrap(err, "temp file")
			}
			defer os.Remove(tempFile.Name())

			_, err = tempFile.WriteString(strings.Replace(string(out), "##CIDR##", cmd.CIDR, -1))
			if err != nil {
				return errors.Wrap(err, "write values to temp file")
			}

			err = tempFile.Close()
			if err != nil {
				return errors.Wrap(err, "close temp file")
			}

			extraValues = append(extraValues, tempFile.Name())
		}
	}

	workDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("unable to get current work directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workDir, cmd.ChartName)); err == nil {
		return fmt.Errorf("aborting vcluster creation. Current working directory contains a file or a directory with the name equal to the vcluster chart name - \"%s\". Please execute vcluster create command from a directory that doesn't contain a file or directory named \"%s\"", cmd.ChartName, cmd.ChartName)
	}

	// we have to upgrade / install the chart
	err = helm.NewClient(&rawConfig, cmd.log).Upgrade(args[0], cmd.Namespace, helm.UpgradeOptions{
		Chart:       cmd.ChartName,
		Path:        cmd.LocalChartDir,
		Repo:        cmd.ChartRepo,
		Version:     cmd.ChartVersion,
		Values:      chartValues,
		ValuesFiles: extraValues,
	})
	if err != nil {
		return err
	}

	cmd.log.Donef("Successfully created virtual cluster %s in namespace %s. \n- Use 'vcluster connect %s --namespace %s' to access the virtual cluster\n- Use `vcluster connect %s --namespace %s -- kubectl get ns` to run a command directly within the vcluster", args[0], cmd.Namespace, args[0], cmd.Namespace, args[0], cmd.Namespace)

	// check if we should connect to the vcluster
	if cmd.Connect {
		connectCmd := &ConnectCmd{
			GlobalFlags: cmd.GlobalFlags,
			KubeConfig:  "./kubeconfig.yaml",
			LocalPort:   8443,
			Log:         cmd.log,
		}

		// TODO: allow commands here as well?
		return connectCmd.Connect(args[0], nil)
	}
	return nil
}

func (cmd *CreateCmd) ToChartOptions(kubernetesVersion *version.Info) (*helm.ChartOptions, error) {
	if !util.Contains(cmd.Distro, AllowedDistros) {
		return nil, fmt.Errorf("unsupported distro %s, please select one of: %s", cmd.Distro, strings.Join(AllowedDistros, ", "))
	}

	if cmd.ChartName == "vcluster" && cmd.Distro != "k3s" {
		cmd.ChartName += "-" + cmd.Distro
	}

	return &helm.ChartOptions{
		ChartName:          cmd.ChartName,
		ChartRepo:          cmd.ChartRepo,
		ChartVersion:       cmd.ChartVersion,
		CIDR:               cmd.CIDR,
		CreateClusterRole:  cmd.CreateClusterRole,
		DisableIngressSync: cmd.DisableIngressSync,
		Expose:             cmd.Expose,
		K3SImage:           cmd.K3SImage,
		Isolate:            cmd.Isolate,
		KubernetesVersion:  kubernetesVersion,
	}, nil
}
