package cluster

import (
	"context"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/giantswarm/apiextensions/v3/pkg/id"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/giantswarm/microerror"
	"github.com/giantswarm/micrologger"
	"github.com/spf13/cobra"

	"github.com/giantswarm/kubectl-gs/cmd/template/cluster/provider"
	"github.com/giantswarm/kubectl-gs/pkg/commonconfig"
	"github.com/giantswarm/kubectl-gs/pkg/labels"

	"github.com/giantswarm/kubectl-gs/internal/key"
)

const (
	clusterCRFileName = "clusterCR"
)

type runner struct {
	flag   *flag
	logger micrologger.Logger
	stdout io.Writer
	stderr io.Writer
}

func (r *runner) Run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Sorting is required before validation for uniqueness.
	sort.Slice(r.flag.ControlPlaneAZ, func(i, j int) bool {
		return r.flag.ControlPlaneAZ[i] < r.flag.ControlPlaneAZ[j]
	})

	err := r.flag.Validate()
	if err != nil {
		return microerror.Mask(err)
	}

	err = r.run(ctx, cmd, args)
	if err != nil {
		return microerror.Mask(err)
	}

	return nil
}

func (r *runner) run(ctx context.Context, cmd *cobra.Command, args []string) error {
	var err error

	var config provider.ClusterCRsConfig
	{
		config = provider.ClusterCRsConfig{
			FileName:           clusterCRFileName,
			ControlPlaneAZ:     r.flag.ControlPlaneAZ,
			ControlPlaneSubnet: r.flag.ControlPlaneSubnet,
			ExternalSNAT:       r.flag.ExternalSNAT,
			EKS:                r.flag.EKS,
			Description:        r.flag.Description,
			Name:               r.flag.Name,
			Organization:       r.flag.Organization,
			PodsCIDR:           r.flag.PodsCIDR,
			ReleaseVersion:     r.flag.Release,
			Namespace:          metav1.NamespaceDefault,
		}

		if len(r.flag.MasterAZ) > 0 {
			config.ControlPlaneAZ = r.flag.MasterAZ
		}

		if config.Name == "" {
			config.Name = id.Generate()
		}

		// Remove leading 'v' from release flag input.
		config.ReleaseVersion = strings.TrimLeft(config.ReleaseVersion, "v")

		config.Labels, err = labels.Parse(r.flag.Label)
		if err != nil {
			return microerror.Mask(err)
		}

		if r.flag.Provider == key.ProviderAzure {
			config.Namespace = key.OrganizationNamespaceFromName(config.Organization)
		}
	}

	commonConfig := commonconfig.New(r.flag.config)
	c, err := commonConfig.GetClient(r.logger)
	if err != nil {
		return microerror.Mask(err)
	}

	var output *os.File
	{
		if r.flag.Output == "" {
			output = os.Stdout
		} else {
			f, err := os.Create(r.flag.Output)
			if err != nil {
				return microerror.Mask(err)
			}
			defer f.Close()

			output = f
		}
	}

	switch r.flag.Provider {
	case key.ProviderAWS:
		err = provider.WriteAWSTemplate(ctx, c.K8sClient, output, config)
		if err != nil {
			return microerror.Mask(err)
		}
	case key.ProviderAzure:
		err = provider.WriteAzureTemplate(ctx, c.K8sClient, output, config)
		if err != nil {
			return microerror.Mask(err)
		}
	}

	return nil
}
