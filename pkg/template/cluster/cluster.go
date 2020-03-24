package cluster

import (
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"time"

	infrastructurev1alpha2 "github.com/giantswarm/apiextensions/pkg/apis/infrastructure/v1alpha2"
	infrastructurev1alpha2scheme "github.com/giantswarm/apiextensions/pkg/clientset/versioned/scheme"
	"github.com/giantswarm/microerror"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/reference"
	apiv1alpha2 "sigs.k8s.io/cluster-api/api/v1alpha2"

	"github.com/giantswarm/kubectl-gs/internal/label"
)

const (
	defaultMasterInstanceType = "m5.xlarge"
	// IDChars represents the character set used to generate cluster IDs.
	// (does not contain 1 and l, to avoid confusion)
	idChars = "023456789abcdefghijkmnopqrstuvwxyz"
	// IDLength represents the number of characters used to create a cluster ID.
	idLength = 5
)

type Config struct {
	Domain            string
	MasterAZ          string
	Name              string
	Owner             string
	Region            string
	ReleaseComponents map[string]string
	ReleaseVersion    string
}

func NewClusterCRs(config Config) (*apiv1alpha2.Cluster, *infrastructurev1alpha2.AWSCluster, error) {

	clusterID := generateID()

	awsClusterCR, err := newAWSClusterCR(clusterID, config)
	if err != nil {
		return nil, nil, microerror.Mask(err)
	}

	clusterCR, err := newClusterCR(awsClusterCR, clusterID, config)
	if err != nil {
		return nil, nil, microerror.Mask(err)
	}

	return clusterCR, awsClusterCR, nil
}

func newClusterCR(obj interface{}, clusterID string, c Config) (*apiv1alpha2.Cluster, error) {
	runtimeObj, _ := obj.(runtime.Object)

	infrastructureCRRef, err := reference.GetReference(infrastructurev1alpha2scheme.Scheme, runtimeObj)
	if err != nil {
		return nil, microerror.Mask(err)
	}

	clusterCR := &apiv1alpha2.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: "cluster.x-k8s.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterID,
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				label.ClusterOperatorVersion: c.ReleaseComponents["cluster-operator"],
				label.Cluster:                clusterID,
				label.Organization:           c.Owner,
				label.ReleaseVersion:         c.ReleaseVersion,
			},
		},
		Spec: apiv1alpha2.ClusterSpec{
			InfrastructureRef: infrastructureCRRef,
		},
	}

	return clusterCR, nil
}

func newAWSClusterCR(clusterID string, c Config) (*infrastructurev1alpha2.AWSCluster, error) {

	awsClusterCR := &infrastructurev1alpha2.AWSCluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "AWSCluster",
			APIVersion: "infrastructure.giantswarm.io/v1alpha2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterID,
			Namespace: metav1.NamespaceDefault,
			Labels: map[string]string{
				label.AWSOperatorVersion: c.ReleaseComponents["aws-operator"],
				label.Cluster:            clusterID,
				label.Organization:       c.Owner,
				label.ReleaseVersion:     c.ReleaseVersion,
			},
		},
		Spec: infrastructurev1alpha2.AWSClusterSpec{
			Cluster: infrastructurev1alpha2.AWSClusterSpecCluster{
				Description: c.Name,
				DNS: infrastructurev1alpha2.AWSClusterSpecClusterDNS{
					Domain: c.Domain,
				},
				OIDC: infrastructurev1alpha2.AWSClusterSpecClusterOIDC{},
			},
			Provider: infrastructurev1alpha2.AWSClusterSpecProvider{
				CredentialSecret: infrastructurev1alpha2.AWSClusterSpecProviderCredentialSecret{
					Name:      "credential-default",
					Namespace: "giantswarm",
				},
				Master: infrastructurev1alpha2.AWSClusterSpecProviderMaster{
					AvailabilityZone: c.MasterAZ,
					InstanceType:     defaultMasterInstanceType,
				},
				Region: c.Region,
			},
		},
	}

	return awsClusterCR, nil
}

func generateID() string {
	for {
		letterRunes := []rune(idChars)
		b := make([]rune, idLength)
		rand.Seed(time.Now().UnixNano())
		for i := range b {
			b[i] = letterRunes[rand.Intn(len(letterRunes))]
		}

		id := string(b)

		if _, err := strconv.Atoi(id); err == nil {
			// string is numbers only, which we want to avoid
			continue
		}

		matched, err := regexp.MatchString("^[a-z]+$", id)
		if err == nil && matched == true {
			// strings is letters only, which we also avoid
			continue
		}
		fmt.Printf("Rendered id: %s\n", id)
		return id
	}
}
