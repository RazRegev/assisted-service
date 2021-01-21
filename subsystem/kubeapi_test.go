package subsystem

import (
	"context"
	"fmt"
	"github.com/jinzhu/gorm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/assisted-service/internal/common"
	"github.com/openshift/assisted-service/internal/controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func deployPullSecretResource(ctx context.Context, client k8sclient.Client, name, secret string) {
	data := map[string][]byte{"pullSecret": []byte(secret)}
	err := client.Create(ctx, &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: getAPIVersion(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Options.Namespace,
			Name:      name,
		},
		Data: data,
	})
	Expect(err).ShouldNot(HaveOccurred())
}

func deleteAllPullSecretsResources(ctx context.Context, client k8sclient.Client) {
	secretList := &corev1.SecretList{}
	err := client.List(ctx, secretList)
	Expect(err).ShouldNot(HaveOccurred())

	for _, secret := range secretList.Items {
		deletePullSecretResource(ctx, client, &secret)
	}
}

func deletePullSecretResource(ctx context.Context, client k8sclient.Client, secret *corev1.Secret) {
	err := client.Delete(ctx, secret)
	Expect(err).ShouldNot(HaveOccurred())
}

func getDefaultClusterSpec() *v1alpha1.ClusterSpec {
	return &v1alpha1.ClusterSpec{
		Name:                     "test-cluster",
		OpenshiftVersion:         "4.6",
		BaseDNSDomain:            "test.domain",
		ClusterNetworkCidr:       "192.168.126.0/14",
		ClusterNetworkHostPrefix: 14,
		ServiceNetworkCidr:       "172.0.0.0/16",
		IngressVip:               "192.168.126.100",
		SSHPublicKey:             sshPublicKey,
		VIPDhcpAllocation:        false,
		HTTPProxy:                "http://10.10.1.1:3128",
		HTTPSProxy:               "https://10.10.1.1:3129",
		NoProxy:                  "quay.io",
		UserManagedNetworking:    false,
		AdditionalNtpSource:      "test.ntp.source",
		PullSecretRef:            secretRef,
	}
}
func deployClusterCRD(ctx context.Context, client k8sclient.Client, spec v1alpha1.ClusterSpec) {
	err := client.Create(ctx, &v1alpha1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: getAPIVersion(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: Options.Namespace,
			Name:      spec.Name,
		},
		Spec: spec,
	})
	Expect(err).ShouldNot(HaveOccurred())
}

func getAPIVersion() string {
	return fmt.Sprintf("%s/%s", v1alpha1.GroupVersion.Group, v1alpha1.GroupVersion.Version)
}

func getClusterFromDB(
	ctx context.Context, client k8sclient.Client, db *gorm.DB, key types.NamespacedName, timeout int) *common.Cluster {
	var err error
	cluster := &common.Cluster{}
	start := time.Now()
	for time.Duration(timeout)*time.Second > time.Since(start) {
		err = db.Take(&cluster, "kube_key_name = ? and kube_key_namespace = ?", key.Name, key.Namespace).Error
		if err == nil {
			return cluster
		}
		if !gorm.IsRecordNotFoundError(err) {
			Expect(err).ShouldNot(HaveOccurred())
		}
		clusterCRD := getClusterCRD(ctx, client, key)
		Expect(clusterCRD.Status.Error).Should(Equal(""))
		time.Sleep(time.Second)
	}
	Expect(err).ShouldNot(HaveOccurred())
	return cluster
}

func getClusterCRD(ctx context.Context, client k8sclient.Client, key types.NamespacedName) *v1alpha1.Cluster {
	cluster := &v1alpha1.Cluster{}
	err := client.Get(ctx, key, cluster)
	Expect(err).ShouldNot(HaveOccurred())
	return cluster
}

func deleteAllClustersCRDs(ctx context.Context, client k8sclient.Client) {
	clusterList := &v1alpha1.ClusterList{}
	err := client.List(ctx, clusterList)
	Expect(err).ShouldNot(HaveOccurred())

	for _, cluster := range clusterList.Items {
		deleteClusterCRD(ctx, client, &cluster)
	}
}

func deleteClusterCRD(ctx context.Context, client k8sclient.Client, cluster *v1alpha1.Cluster) {
	err := client.Delete(ctx, cluster)
	Expect(err).ShouldNot(HaveOccurred())
}

var _ = Describe("input/output tests", func() {
	ctx := context.Background()
	client := kubeClient.Client

	deleteAllPullSecretsResources(ctx, client)
	deleteAllClustersCRDs(ctx, client)

	Context("cluster", func() {
		secretRef := &corev1.SecretReference{}

		BeforeSuite(func() {
			deployPullSecretResource(ctx, client, "pull-secret", pullSecret)
			secretRef = &corev1.SecretReference{
				Namespace: Options.Namespace,
				Name:      "pull-secret",
			}
		})
		AfterEach(func() {
			deleteAllClustersCRDs(ctx, client)
			clearDB()
		})

		It("deploy cluster and verify in db", func() {
			spec := getDefaultClusterSpec()
			deployClusterCRD(ctx, client, *spec)
			key := types.NamespacedName{
				Namespace: Options.Namespace,
				Name:      spec.Name,
			}
			cluster := getClusterFromDB(ctx, client, db, key, 10)
			Expect(cluster.Name).Should(Equal(spec.Name))
			Expect(cluster.OpenshiftVersion).Should(Equal(spec.OpenshiftVersion))
			Expect(cluster.BaseDNSDomain).Should(Equal(spec.BaseDNSDomain))
			Expect(cluster.ClusterNetworkCidr).Should(Equal(spec.ClusterNetworkCidr))
			Expect(cluster.ClusterNetworkHostPrefix).Should(Equal(spec.ClusterNetworkHostPrefix))
			Expect(cluster.ServiceNetworkCidr).Should(Equal(spec.ServiceNetworkCidr))
			Expect(cluster.IngressVip).Should(Equal(spec.IngressVip))
			Expect(cluster.SSHPublicKey).Should(Equal(spec.SSHPublicKey))
			Expect(cluster.VipDhcpAllocation).Should(Equal(spec.VIPDhcpAllocation))
			Expect(cluster.HTTPProxy).Should(Equal(spec.HTTPProxy))
			Expect(cluster.HTTPSProxy).Should(Equal(spec.HTTPSProxy))
			Expect(cluster.NoProxy).Should(Equal(spec.NoProxy))
			Expect(cluster.UserManagedNetworking).Should(Equal(spec.UserManagedNetworking))
			Expect(cluster.OpenshiftVersion).Should(Equal(spec.OpenshiftVersion))
			Expect(cluster.AdditionalNtpSource).Should(Equal(spec.AdditionalNtpSource))
			Expect(cluster.PullSecret).Should(Equal(pullSecret))
		})
	})
	//todo: deletion of pull secret - verify deletion in db
})
