package apiclient

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/jinzhu/gorm"
	"github.com/openshift/assisted-service/client/installer"
	"github.com/openshift/assisted-service/internal/common"
	adiiov1alpha1 "github.com/openshift/assisted-service/internal/controller/api/v1alpha1"
	"github.com/openshift/assisted-service/models"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type KubeAPIClient struct {
	client    runtimeclient.Client
	namespace string
	db        *gorm.DB
}

func NewKubeAPIClient(
	namespace string,
	config *rest.Config,
	db *gorm.DB,
) (*KubeAPIClient, error) {

	client, err := runtimeclient.New(config, runtimeclient.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}
	kc := &KubeAPIClient{
		client:    client,
		namespace: namespace,
		db:        db,
	}
	return kc, nil
}

func (kc *KubeAPIClient) RegisterCluster(
	ctx context.Context,
	params *installer.RegisterClusterParams,
) (*models.Cluster, error) {

	pullSecretRef, pullSecretErr := kc.GetSecretRefAndDeployIfNotExists(
		ctx,
		"pull-secret",
		swag.StringValue(params.NewClusterParams.PullSecret))
	if pullSecretErr != nil {
		return nil, errors.Wrapf(pullSecretErr, "failed to deploy pull secret")
	}

	c := adiiov1alpha1.Cluster{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Cluster",
			APIVersion: getAPIVersion(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      swag.StringValue(params.NewClusterParams.Name),
			Namespace: kc.namespace,
		},
		Spec: adiiov1alpha1.ClusterSpec{
			Name:                     swag.StringValue(params.NewClusterParams.Name),
			OpenshiftVersion:         swag.StringValue(params.NewClusterParams.OpenshiftVersion),
			BaseDNSDomain:            swag.StringValue(&params.NewClusterParams.BaseDNSDomain),
			ClusterNetworkCidr:       swag.StringValue(params.NewClusterParams.ClusterNetworkCidr),
			ClusterNetworkHostPrefix: swag.Int64Value(&params.NewClusterParams.ClusterNetworkHostPrefix),
			ServiceNetworkCidr:       swag.StringValue(params.NewClusterParams.ServiceNetworkCidr),
			IngressVip:               swag.StringValue(&params.NewClusterParams.IngressVip),
			SSHPublicKey:             swag.StringValue(&params.NewClusterParams.SSHPublicKey),
			VIPDhcpAllocation:        swag.BoolValue(params.NewClusterParams.VipDhcpAllocation),
			HTTPProxy:                swag.StringValue(params.NewClusterParams.HTTPProxy),
			HTTPSProxy:               swag.StringValue(params.NewClusterParams.HTTPSProxy),
			NoProxy:                  swag.StringValue(params.NewClusterParams.NoProxy),
			UserManagedNetworking:    swag.BoolValue(params.NewClusterParams.UserManagedNetworking),
			AdditionalNtpSource:      swag.StringValue(params.NewClusterParams.AdditionalNtpSource),
			PullSecretRef:            pullSecretRef,
		},
	}
	if deployErr := kc.client.Create(ctx, &c); deployErr != nil {
		return nil, errors.Wrapf(deployErr, "failed to deploy cluster")
	}

	return kc.getClusterWithRetries(ctx, types.NamespacedName{
		Name:      swag.StringValue(params.NewClusterParams.Name),
		Namespace: kc.namespace,
	})
}

func (kc *KubeAPIClient) GetSecretRefAndDeployIfNotExists(
	ctx context.Context,
	secretName, pullSecret string,
) (*corev1.SecretReference, error) {

	ref := &corev1.SecretReference{
		Name:      secretName,
		Namespace: kc.namespace,
	}
	if pullSecret == "" {
		return ref, nil
	}
	key := types.NamespacedName{
		Name:      secretName,
		Namespace: kc.namespace,
	}
	if _, err := kc.getSecret(ctx, key); runtimeclient.IgnoreNotFound(err) != nil {
		return nil, err
	} else if err == nil {
		return ref, nil
	}
	return kc.deployPullSecret(ctx, secretName, pullSecret)
}

func (kc *KubeAPIClient) deployPullSecret(
	ctx context.Context,
	name, secret string,
) (*corev1.SecretReference, error) {

	if secret == "" {
		return nil, nil
	}

	if err := kc.client.Create(ctx, &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: getAPIVersion(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: kc.namespace,
		},
		StringData: map[string]string{"pullSecret": secret},
	}); err != nil {
		return nil, err
	}

	return &corev1.SecretReference{
		Name:      name,
		Namespace: kc.namespace,
	}, nil
}

func (kc *KubeAPIClient) getSecret(ctx context.Context, key types.NamespacedName) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := kc.client.Get(ctx, key, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func (kc *KubeAPIClient) getClusterWithRetries(
	ctx context.Context,
	key types.NamespacedName,
) (*models.Cluster, error) {

	var cluster *models.Cluster
	var getClusterErr error
	for i := 0; i < 5; i++ {
		if verifyStatusErr := kc.verifyClusterStatus(ctx, key); verifyStatusErr != nil {
			return nil, errors.Wrapf(verifyStatusErr, "bad cluster status")
		}
		if cluster, getClusterErr = kc.getClusterByKeyFromDB(key); getClusterErr == gorm.ErrRecordNotFound {
			time.Sleep(time.Millisecond * 500)
			continue
		}
		break
	}
	if getClusterErr != nil {
		return nil, errors.Wrapf(getClusterErr, "failed to get cluster")
	}
	return cluster, nil
}

func (kc *KubeAPIClient) verifyClusterStatus(ctx context.Context, key types.NamespacedName) error {
	cluster := &adiiov1alpha1.Cluster{}
	if err := kc.client.Get(ctx, key, cluster); err != nil {
		return err
	}
	if cluster.Status.Error != "" {
		return errors.Errorf("cluster status error: %s", cluster.Status.Error)
	}
	return nil
}

func (kc *KubeAPIClient) getClusterByKeyFromDB(key types.NamespacedName) (*models.Cluster, error) {
	c := &common.Cluster{}
	if res := kc.db.Take(&c, "kube_key_name = ? and kube_key_namespace = ?",
		key.Name, key.Namespace); res.Error != nil {
		return nil, res.Error
	}
	return &c.Cluster, nil
}

func (kc *KubeAPIClient) DeregisterCluster(
	ctx context.Context,
	params *installer.DeregisterClusterParams,
) error {

	cluster, getErr := kc.getClusterByID(params.ClusterID)
	if getErr != nil {
		return errors.Wrapf(getErr, "failed getting cluster from db")
	}

	if delErr := kc.client.Delete(ctx, &adiiov1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: kc.namespace,
		},
	}); delErr != nil {
		return errors.Wrapf(delErr, "failed deleting cluster")
	}

	// todo: delete this part when db deletion will be a part of the cluster controller
	if dbErr := kc.db.Delete(&common.Cluster{}, "id = ?", params.ClusterID.String()).Error; dbErr != nil {
		return errors.Wrapf(dbErr, "failed deleting cluster from db")
	}
	return nil
}

func (kc *KubeAPIClient) getClusterByID(id strfmt.UUID) (*common.Cluster, error) {
	c := &common.Cluster{}
	if res := kc.db.Take(c, "id = ?", id.String()); res.Error != nil {
		return nil, res.Error
	} else if res.RowsAffected == 0 {
		return nil, errors.New("cluster was not found in db")
	}
	return c, nil
}

func (kc *KubeAPIClient) UpdateCluster(
	ctx context.Context,
	params *installer.UpdateClusterParams,
) (*models.Cluster, error) {

	cluster, getClusterErr := kc.getClusterByID(params.ClusterID)
	if getClusterErr != nil {
		return nil, errors.Wrapf(getClusterErr, "failed to get cluster")
	}
	spec, specErr := kc.buildClusterUpdateSpec(ctx, cluster, params.ClusterUpdateParams)
	if specErr != nil {
		return nil, errors.Wrapf(specErr, "failed build cluster update spec")
	}
	patch, patchErr := kc.createPatchFromSpec(spec)
	if patchErr != nil {
		return nil, errors.Wrapf(patchErr, "failed get patch from spec")
	}

	if deployPatchErr := kc.client.Patch(ctx,
		&adiiov1alpha1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cluster.Name,
				Namespace: kc.namespace,
			},
		},
		runtimeclient.RawPatch(types.MergePatchType, patch)); deployPatchErr != nil {
		return nil, errors.Wrapf(deployPatchErr, "failed to deploy cluster patch %s", patch)
	}
	return kc.getClusterAfterUpdateWithRetries(ctx, cluster.Cluster, spec)
}

func (kc *KubeAPIClient) buildClusterUpdateSpec(
	ctx context.Context,
	cluster *common.Cluster,
	params *models.ClusterUpdateParams,
) (*adiiov1alpha1.ClusterSpec, error) {

	setString := func(new *string, old string, target *string) {
		if new != nil && swag.StringValue(new) != "" && *new != old {
			*target = swag.StringValue(new)
		}
	}
	setBool := func(new *bool, old bool, target *bool) {
		if new != nil && *new != old {
			*target = swag.BoolValue(new)
		}
	}
	setInt64 := func(new *int64, old int64, target *int64) {
		if new != nil && *new != old {
			*target = swag.Int64Value(new)
		}
	}

	spec := adiiov1alpha1.ClusterSpec{}
	setString(params.AdditionalNtpSource, cluster.AdditionalNtpSource, &spec.AdditionalNtpSource)
	setString(params.APIVip, cluster.APIVip, &spec.APIVip)
	setString(params.APIVipDNSName, swag.StringValue(cluster.APIVipDNSName), &spec.APIVipDNSName)
	setString(params.BaseDNSDomain, cluster.BaseDNSDomain, &spec.BaseDNSDomain)
	setString(params.ClusterNetworkCidr, cluster.ClusterNetworkCidr, &spec.ClusterNetworkCidr)
	setString(params.HTTPProxy, cluster.HTTPProxy, &spec.HTTPProxy)
	setString(params.HTTPSProxy, cluster.HTTPSProxy, &spec.HTTPSProxy)
	setString(params.IngressVip, cluster.IngressVip, &spec.IngressVip)
	setString(params.MachineNetworkCidr, cluster.MachineNetworkCidr, spec.MachineNetworkCidr)
	setString(params.Name, cluster.Name, &spec.Name)
	setString(params.NoProxy, cluster.NoProxy, &spec.NoProxy)
	setString(params.ServiceNetworkCidr, cluster.ServiceNetworkCidr, &spec.ServiceNetworkCidr)
	setString(params.SSHPublicKey, cluster.SSHPublicKey, &spec.SSHPublicKey)
	setBool(params.UserManagedNetworking, swag.BoolValue(cluster.UserManagedNetworking), &spec.UserManagedNetworking)
	setBool(params.VipDhcpAllocation, swag.BoolValue(cluster.VipDhcpAllocation), &spec.VIPDhcpAllocation)
	setInt64(params.ClusterNetworkHostPrefix, cluster.ClusterNetworkHostPrefix, &spec.ClusterNetworkHostPrefix)

	spec.OpenshiftVersion = cluster.OpenshiftVersion

	pullSecret := cluster.PullSecret
	if params.PullSecret != nil && swag.StringValue(params.PullSecret) != "" {
		pullSecret = swag.StringValue(params.PullSecret)
	}
	pullSecretRef, pullSecretErr := kc.GetSecretRefAndDeployIfNotExists(
		ctx,
		spec.Name,
		pullSecret)
	if pullSecretErr != nil {
		return nil, errors.Wrapf(pullSecretErr, "failed to deploy pull secret")
	}
	spec.PullSecretRef = pullSecretRef
	return &spec, nil
}

func (kc *KubeAPIClient) createPatchFromSpec(spec *adiiov1alpha1.ClusterSpec) ([]byte, error) {
	if patch, err := json.Marshal(spec); err != nil {
		return nil, err
	} else {
		return []byte(fmt.Sprintf(`{"spec": %s}`, patch)), nil
	}
}

func (kc *KubeAPIClient) getClusterAfterUpdateWithRetries(
	ctx context.Context,
	cluster models.Cluster,
	spec *adiiov1alpha1.ClusterSpec,
) (*models.Cluster, error) {

	var c *models.Cluster
	var err error
	for i := 0; i < 5; i++ {
		if verifyStatusErr := kc.verifyClusterStatus(ctx, types.NamespacedName{
			Name:      spec.Name,
			Namespace: kc.namespace,
		}); verifyStatusErr != nil {
			return nil, errors.Wrapf(verifyStatusErr, "bad cluster status")
		}
		c, err = kc.getClusterByKeyFromDB(types.NamespacedName{
			Name:      spec.Name,
			Namespace: kc.namespace,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to get cluster after patch")
		}
		if cluster.UpdatedAt != c.UpdatedAt {
			return c, nil
		}
		time.Sleep(time.Millisecond * 500)
	}
	return nil, errors.New("cluster has failed to be updated")
}

func (kc *KubeAPIClient) DeleteAllClusters(ctx context.Context) error {
	cl := &adiiov1alpha1.ClusterList{}
	if err := kc.client.List(ctx, cl); err != nil {
		return errors.Wrapf(err, "failed listing clusters")
	}
	for _, c := range cl.Items {
		if err := kc.client.Delete(ctx, &c); err != nil {
			return errors.Wrapf(err, "failed deleting cluster")
		}
	}
	return nil
}

func getAPIVersion() string {
	return fmt.Sprintf("%s/%s", adiiov1alpha1.GroupVersion.Group, adiiov1alpha1.GroupVersion.Version)
}
