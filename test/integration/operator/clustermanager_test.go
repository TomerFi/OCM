package operator

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/cert"

	"open-cluster-management.io/api/feature"
	operatorapiv1 "open-cluster-management.io/api/operator/v1"

	"open-cluster-management.io/ocm/pkg/operator/helpers"
	"open-cluster-management.io/ocm/pkg/operator/operators/clustermanager"
	certrotation "open-cluster-management.io/ocm/pkg/operator/operators/clustermanager/controllers/certrotationcontroller"
	"open-cluster-management.io/ocm/test/integration/util"
)

const (
	testImage      = "testimage:latest"
	infraNodeLabel = "node-role.kubernetes.io/infra"
)

func startHubOperator(ctx context.Context, mode operatorapiv1.InstallMode) {
	certrotation.SigningCertValidity = time.Second * 30
	certrotation.TargetCertValidity = time.Second * 10
	certrotation.ResyncInterval = time.Second * 1

	var config *rest.Config
	switch mode {
	case operatorapiv1.InstallModeDefault:
		config = restConfig
	case operatorapiv1.InstallModeHosted:
		config = hostedRestConfig
	}

	o := &clustermanager.Options{}
	err := o.RunClusterManagerOperator(ctx, &controllercmd.ControllerContext{
		KubeConfig:    config,
		EventRecorder: util.NewIntegrationTestEventRecorder("integration"),
	})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())
}

var _ = ginkgo.Describe("ClusterManager Default Mode", func() {
	var cancel context.CancelFunc
	var hubRegistrationDeployment = fmt.Sprintf("%s-registration-controller", clusterManagerName)
	var hubPlacementDeployment = fmt.Sprintf("%s-placement-controller", clusterManagerName)
	var hubRegistrationWebhookDeployment = fmt.Sprintf("%s-registration-webhook", clusterManagerName)
	var hubWorkWebhookDeployment = fmt.Sprintf("%s-work-webhook", clusterManagerName)
	var hubAddOnManagerDeployment = fmt.Sprintf("%s-addon-manager-controller", clusterManagerName)
	var hubWorkControllerDeployment = fmt.Sprintf("%s-work-controller", clusterManagerName)
	var hubAddonManagerDeployment = fmt.Sprintf("%s-addon-manager-controller", clusterManagerName)
	var hubRegistrationClusterRole = fmt.Sprintf("open-cluster-management:%s-registration:controller", clusterManagerName)
	var hubRegistrationWebhookClusterRole = fmt.Sprintf("open-cluster-management:%s-registration:webhook", clusterManagerName)
	var hubWorkWebhookClusterRole = fmt.Sprintf("open-cluster-management:%s-work:webhook", clusterManagerName)
	var hubWorkControllerClusterRole = fmt.Sprintf("open-cluster-management:%s-work:controller", clusterManagerName)
	var hubAddOnManagerClusterRole = fmt.Sprintf("open-cluster-management:%s-addon-manager:controller", clusterManagerName)
	var hubRegistrationSA = "registration-controller-sa"
	var hubRegistrationWebhookSA = "registration-webhook-sa"
	var hubWorkWebhookSA = "work-webhook-sa"
	var hubWorkControllerSA = "work-controller-sa"
	var hubAddOnManagerSA = "addon-manager-controller-sa"

	ginkgo.BeforeEach(func() {
		var ctx context.Context
		ctx, cancel = context.WithCancel(context.Background())
		go startHubOperator(ctx, operatorapiv1.InstallModeDefault)
	})

	ginkgo.AfterEach(func() {
		// delete deployment for clustermanager here so tests are not impacted with each other
		err := kubeClient.AppsV1().Deployments(hubNamespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		if cancel != nil {
			cancel()
		}
	})

	ginkgo.Context("Deploy and clean hub component", func() {
		ginkgo.It("should have expected resource created successfully", func() {
			// Check namespace
			gomega.Eventually(func() error {
				if _, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), hubNamespace, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			// Check clusterrole/clusterrolebinding
			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoles().Get(context.Background(), hubRegistrationClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoles().Get(context.Background(), hubRegistrationWebhookClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoles().Get(context.Background(), hubWorkWebhookClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoles().Get(context.Background(), hubWorkControllerClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), hubRegistrationClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), hubRegistrationWebhookClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), hubWorkWebhookClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), hubWorkControllerClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check service account
			gomega.Eventually(func() error {
				if _, err := kubeClient.CoreV1().ServiceAccounts(hubNamespace).Get(context.Background(), hubRegistrationSA, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			gomega.Eventually(func() error {
				if _, err := kubeClient.CoreV1().ServiceAccounts(hubNamespace).Get(context.Background(), hubRegistrationWebhookSA, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			gomega.Eventually(func() error {
				if _, err := kubeClient.CoreV1().ServiceAccounts(hubNamespace).Get(context.Background(), hubWorkWebhookSA, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			gomega.Eventually(func() error {
				if _, err := kubeClient.CoreV1().ServiceAccounts(hubNamespace).Get(context.Background(), hubWorkControllerSA, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check deployment
			gomega.Eventually(func() error {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationWebhookDeployment, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubWorkWebhookDeployment, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubWorkControllerDeployment, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubAddonManagerDeployment, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check service
			gomega.Eventually(func() error {
				if _, err := kubeClient.CoreV1().Services(hubNamespace).Get(context.Background(), "cluster-manager-registration-webhook", metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				if _, err := kubeClient.CoreV1().Services(hubNamespace).Get(context.Background(), "cluster-manager-work-webhook", metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check webhook secret
			registrationWebhookSecret := "registration-webhook-serving-cert"
			gomega.Eventually(func() error {
				s, err := kubeClient.CoreV1().Secrets(hubNamespace).Get(context.Background(), registrationWebhookSecret, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if s.Data == nil {
					return fmt.Errorf("s.Data is nil")
				} else if s.Data["tls.crt"] == nil {
					return fmt.Errorf("s.Data doesn't contain key 'tls.crt'")
				} else if s.Data["tls.key"] == nil {
					return fmt.Errorf("s.Data doesn't contain key 'tls.key'")
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			//#nosec G101
			workWebhookSecret := "work-webhook-serving-cert"
			gomega.Eventually(func() error {
				s, err := kubeClient.CoreV1().Secrets(hubNamespace).Get(context.Background(), workWebhookSecret, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if s.Data == nil {
					return fmt.Errorf("s.Data is nil")
				} else if s.Data["tls.crt"] == nil {
					return fmt.Errorf("s.Data doesn't contain key 'tls.crt'")
				} else if s.Data["tls.key"] == nil {
					return fmt.Errorf("s.Data doesn't contain key 'tls.key'")
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			ginkgo.By("Update the deployment status to fail to prevent other cases from interfering")
			updateDeploymentsStatusFail(kubeClient, hubNamespace,
				hubRegistrationDeployment, hubPlacementDeployment, hubRegistrationWebhookDeployment,
				hubWorkWebhookDeployment, hubWorkControllerDeployment, hubAddonManagerDeployment)

			// Check validating webhook
			registrationValidtingWebhook := "managedclustervalidators.admission.cluster.open-cluster-management.io"
			// Should not apply the webhook config if the replica and observed is not set
			_, err := kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
				context.Background(), registrationValidtingWebhook, metav1.GetOptions{})
			gomega.Expect(err).To(gomega.HaveOccurred())

			workValidtingWebhook := "manifestworkvalidators.admission.work.open-cluster-management.io"
			// Should not apply the webhook config if the replica and observed is not set
			_, err = kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
				context.Background(), workValidtingWebhook, metav1.GetOptions{})
			gomega.Expect(err).To(gomega.HaveOccurred())

			// Update ready replica of deployment
			updateDeploymentsStatusSuccess(kubeClient, hubNamespace,
				hubRegistrationDeployment, hubPlacementDeployment, hubRegistrationWebhookDeployment,
				hubWorkWebhookDeployment, hubWorkControllerDeployment, hubAddonManagerDeployment)

			gomega.Eventually(func() error {
				_, err := kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
					context.Background(), registrationValidtingWebhook, metav1.GetOptions{})
				return err
			}, eventuallyTimeout*10, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				_, err := kubeClient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
					context.Background(), workValidtingWebhook, metav1.GetOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			util.AssertClusterManagerCondition(clusterManagerName, operatorClient, "Applied", "ClusterManagerApplied", metav1.ConditionTrue)
		})

		ginkgo.It("should have expected resource created/deleted when feature gates manifestwork replicaset enabled/disabled", func() {
			ginkgo.By("Disable manifestwork replicaset feature gate")
			// Check manifestwork replicaset disable
			gomega.Eventually(func() error {
				clusterManager, err := operatorClient.OperatorV1().ClusterManagers().Get(
					context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				featureGate := []operatorapiv1.FeatureGate{
					{
						Feature: string(feature.ManifestWorkReplicaSet),
						Mode:    operatorapiv1.FeatureGateModeTypeDisable,
					},
				}
				if clusterManager.Spec.WorkConfiguration != nil {
					for _, fg := range clusterManager.Spec.WorkConfiguration.FeatureGates {
						if fg.Feature != string(feature.ManifestWorkReplicaSet) {
							featureGate = append(featureGate, fg)
						}
					}
				}
				clusterManager.Spec.WorkConfiguration = &operatorapiv1.WorkConfiguration{
					FeatureGates: featureGate,
				}
				_, err = operatorClient.OperatorV1().ClusterManagers().Update(
					context.Background(), clusterManager, metav1.UpdateOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check clusterrole/clusterrolebinding
			gomega.Eventually(func() bool {
				_, err := kubeClient.RbacV1().ClusterRoles().Get(
					context.Background(), hubWorkControllerClusterRole, metav1.GetOptions{})
				if err == nil {
					return false
				}
				return errors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
			gomega.Eventually(func() bool {
				_, err := kubeClient.RbacV1().ClusterRoleBindings().Get(
					context.Background(), hubWorkControllerClusterRole, metav1.GetOptions{})
				if err == nil {
					return false
				}
				return errors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Check service account
			gomega.Eventually(func() bool {
				_, err := kubeClient.CoreV1().ServiceAccounts(hubNamespace).Get(
					context.Background(), hubWorkControllerSA, metav1.GetOptions{})
				if err == nil {
					return false
				}
				return errors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Check deployment
			gomega.Eventually(func() bool {
				_, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(
					context.Background(), hubWorkControllerDeployment, metav1.GetOptions{})
				if err == nil {
					return false
				}
				return errors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			updateDeploymentsStatusSuccess(kubeClient, hubNamespace,
				hubRegistrationDeployment, hubPlacementDeployment, hubRegistrationWebhookDeployment,
				hubWorkWebhookDeployment, hubAddOnManagerDeployment)

			// Check if relatedResources are correct
			gomega.Eventually(func() error {
				actual, err := operatorClient.OperatorV1().ClusterManagers().Get(
					context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if len(actual.Status.RelatedResources) != 42 {
					return fmt.Errorf("should get 42 relatedResources, actual got %v, %v",
						len(actual.Status.RelatedResources), actual.Status.RelatedResources)
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

			ginkgo.By("Revert manifestwork replicaset to enable mode")
			gomega.Eventually(func() error {
				clusterManager, err := operatorClient.OperatorV1().ClusterManagers().Get(
					context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				featureGate := []operatorapiv1.FeatureGate{
					{
						Feature: string(feature.ManifestWorkReplicaSet),
						Mode:    operatorapiv1.FeatureGateModeTypeEnable,
					},
				}
				if clusterManager.Spec.WorkConfiguration != nil {
					for _, fg := range clusterManager.Spec.WorkConfiguration.FeatureGates {
						if fg.Feature != string(feature.ManifestWorkReplicaSet) {
							featureGate = append(featureGate, fg)
						}
					}
				}
				clusterManager.Spec.WorkConfiguration = &operatorapiv1.WorkConfiguration{
					FeatureGates: featureGate,
				}
				_, err = operatorClient.OperatorV1().ClusterManagers().Update(
					context.Background(), clusterManager, metav1.UpdateOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check clusterrole/clusterrolebinding
			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoles().Get(
					context.Background(), hubWorkControllerClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoleBindings().Get(
					context.Background(), hubWorkControllerClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check service account
			gomega.Eventually(func() error {
				if _, err := kubeClient.CoreV1().ServiceAccounts(hubNamespace).Get(
					context.Background(), hubWorkControllerSA, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check deployment
			gomega.Eventually(func() error {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(
					context.Background(), hubWorkControllerDeployment, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			updateDeploymentsStatusSuccess(kubeClient, hubNamespace,
				hubRegistrationDeployment, hubPlacementDeployment, hubRegistrationWebhookDeployment,
				hubWorkWebhookDeployment, hubWorkControllerDeployment, hubAddonManagerDeployment)
			// Check if relatedResources are correct
			gomega.Eventually(func() error {
				actual, err := operatorClient.OperatorV1().ClusterManagers().Get(
					context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if len(actual.Status.RelatedResources) != 46 {
					return fmt.Errorf("should get 46 relatedResources, actual got %v, %v",
						len(actual.Status.RelatedResources), actual.Status.RelatedResources)
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("should have expected resource created/deleted successfully when feature gates AddOnManager enabled/disabled", func() {
			ginkgo.By("Check addon manager disable mode")
			gomega.Eventually(func() error {
				clusterManager, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				clusterManager.Spec.AddOnManagerConfiguration = &operatorapiv1.AddOnManagerConfiguration{
					FeatureGates: []operatorapiv1.FeatureGate{
						{
							Feature: string(feature.AddonManagement),
							Mode:    operatorapiv1.FeatureGateModeTypeDisable,
						},
					},
				}
				_, err = operatorClient.OperatorV1().ClusterManagers().Update(context.Background(), clusterManager, metav1.UpdateOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check clusterrole/clusterrolebinding
			gomega.Eventually(func() bool {
				_, err := kubeClient.RbacV1().ClusterRoles().Get(context.Background(), hubAddOnManagerClusterRole, metav1.GetOptions{})
				if err == nil {
					return false
				}
				return errors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
			gomega.Eventually(func() bool {
				_, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), hubAddOnManagerClusterRole, metav1.GetOptions{})
				if err == nil {
					return false
				}
				return errors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Check service account
			gomega.Eventually(func() bool {
				_, err := kubeClient.CoreV1().ServiceAccounts(hubNamespace).Get(context.Background(), hubAddOnManagerSA, metav1.GetOptions{})
				if err == nil {
					return false
				}
				return errors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			// Check deployment
			gomega.Eventually(func() bool {
				_, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubAddOnManagerDeployment, metav1.GetOptions{})
				if err == nil {
					return false
				}
				return errors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			updateDeploymentsStatusSuccess(kubeClient, hubNamespace,
				hubRegistrationDeployment, hubPlacementDeployment, hubRegistrationWebhookDeployment,
				hubWorkWebhookDeployment, hubWorkControllerDeployment)
			// Check if relatedResources are correct
			gomega.Eventually(func() error {
				actual, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if len(actual.Status.RelatedResources) != 41 {
					return fmt.Errorf("should get 41 relatedResources, actual got %v", len(actual.Status.RelatedResources))
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())

			ginkgo.By("Revert addon manager to enable mode")
			gomega.Eventually(func() error {
				clusterManager, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				clusterManager.Spec.AddOnManagerConfiguration = nil
				_, err = operatorClient.OperatorV1().ClusterManagers().Update(context.Background(), clusterManager, metav1.UpdateOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			// Check clusterrole/clusterrolebinding
			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoles().Get(context.Background(), hubAddOnManagerClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			gomega.Eventually(func() error {
				if _, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), hubAddOnManagerClusterRole, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check service account
			gomega.Eventually(func() error {
				if _, err := kubeClient.CoreV1().ServiceAccounts(hubNamespace).Get(context.Background(), hubAddOnManagerSA, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check deployment
			gomega.Eventually(func() error {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubAddOnManagerDeployment, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			updateDeploymentsStatusSuccess(kubeClient, hubNamespace,
				hubRegistrationDeployment, hubPlacementDeployment, hubRegistrationWebhookDeployment,
				hubWorkWebhookDeployment, hubWorkControllerDeployment, hubAddonManagerDeployment)
			// Check if relatedResources are correct
			gomega.Eventually(func() error {
				actual, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if len(actual.Status.RelatedResources) != 46 {
					return fmt.Errorf("should get 46 relatedResources, actual got %v", len(actual.Status.RelatedResources))
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("Deployment should be updated when clustermanager is changed", func() {
			gomega.Eventually(func() error {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check if generations are correct
			gomega.Eventually(func() error {
				actual, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				if actual.Generation != actual.Status.ObservedGeneration {
					return fmt.Errorf("except generation to be %d, but got %d", actual.Status.ObservedGeneration, actual.Generation)
				}

				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			clusterManager, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			clusterManager.Spec.RegistrationImagePullSpec = testImage
			_, err = operatorClient.OperatorV1().ClusterManagers().Update(context.Background(), clusterManager, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func() error {
				actual, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{})
				if err != nil {
					return err
				}
				gomega.Expect(len(actual.Spec.Template.Spec.Containers)).Should(gomega.Equal(1))
				if actual.Spec.Template.Spec.Containers[0].Image != testImage {
					return fmt.Errorf("expected image to be testimage:latest but get %s", actual.Spec.Template.Spec.Containers[0].Image)
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			updateDeploymentsStatusSuccess(kubeClient, hubNamespace,
				hubRegistrationDeployment, hubPlacementDeployment, hubRegistrationWebhookDeployment,
				hubWorkWebhookDeployment, hubWorkControllerDeployment, hubAddonManagerDeployment)

			// Check if generations are correct
			gomega.Eventually(func() error {
				actual, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				if actual.Generation != actual.Status.ObservedGeneration {
					return fmt.Errorf("except generation to be %d, but got %d", actual.Status.ObservedGeneration, actual.Generation)
				}

				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check if relatedResources are correct
			gomega.Eventually(func() error {
				actual, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if len(actual.Status.RelatedResources) != 46 {
					return fmt.Errorf("should get 46 relatedResources, actual got %v", len(actual.Status.RelatedResources))
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(gomega.HaveOccurred())
		})

		ginkgo.It("Deployment should be added nodeSelector and toleration when add nodePlacement into clustermanager", func() {
			gomega.Eventually(func() error {
				clusterManager, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}
				clusterManager.Spec.NodePlacement = operatorapiv1.NodePlacement{
					NodeSelector: map[string]string{infraNodeLabel: ""},
					Tolerations: []corev1.Toleration{
						{
							Key:      infraNodeLabel,
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				}
				_, err = operatorClient.OperatorV1().ClusterManagers().Update(context.Background(), clusterManager, metav1.UpdateOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				actual, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{})
				if err != nil {
					return err
				}
				gomega.Expect(len(actual.Spec.Template.Spec.Containers)).Should(gomega.Equal(1))
				if len(actual.Spec.Template.Spec.NodeSelector) == 0 {
					return fmt.Errorf("length of node selector should not equals to 0")
				}
				if _, ok := actual.Spec.Template.Spec.NodeSelector[infraNodeLabel]; !ok {
					return fmt.Errorf("node-role.kubernetes.io/infra not exist")
				}
				if len(actual.Spec.Template.Spec.Tolerations) == 0 {
					return fmt.Errorf("length of node selecor should not equals to 0")
				}
				for _, toleration := range actual.Spec.Template.Spec.Tolerations {
					if toleration.Key == infraNodeLabel {
						return nil
					}
				}

				return fmt.Errorf("no key equals to node-role.kubernetes.io/infra")
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			updateDeploymentsStatusSuccess(kubeClient, hubNamespace,
				hubRegistrationDeployment, hubPlacementDeployment, hubRegistrationWebhookDeployment,
				hubWorkWebhookDeployment, hubWorkControllerDeployment, hubAddonManagerDeployment)
		})
		ginkgo.It("Deployment should be reconciled when manually updated", func() {
			gomega.Eventually(func() error {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			registrationoDeployment, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			registrationoDeployment.Spec.Template.Spec.Containers[0].Image = "testimage2:latest"
			_, err = kubeClient.AppsV1().Deployments(hubNamespace).Update(context.Background(), registrationoDeployment, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Eventually(func() error {
				registrationoDeployment, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{})
				if err != nil {
					return err
				}
				if registrationoDeployment.Spec.Template.Spec.Containers[0].Image != testImage {
					return fmt.Errorf("image should be testimage:latest, but get %s", registrationoDeployment.Spec.Template.Spec.Containers[0].Image)
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// Check if generations are correct
			gomega.Eventually(func() error {
				actual, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				registrationDeployment, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{})
				if err != nil {
					return err
				}

				deploymentGeneration := helpers.NewGenerationStatus(appsv1.SchemeGroupVersion.WithResource("deployments"), registrationDeployment)
				actualGeneration := helpers.FindGenerationStatus(actual.Status.Generations, deploymentGeneration)
				if deploymentGeneration.LastGeneration != actualGeneration.LastGeneration {
					return fmt.Errorf("expected LastGeneration shoud be %d, but get %d", actualGeneration.LastGeneration, deploymentGeneration.LastGeneration)
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
		})

		ginkgo.It("should have auto approver user set on registration when configured", func() {
			// Update cluster manager configuration
			gomega.Eventually(func() error {
				clusterManager, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				// Check addon manager enabled mode
				if clusterManager.Spec.RegistrationConfiguration == nil {
					clusterManager.Spec.RegistrationConfiguration = &operatorapiv1.RegistrationHubConfiguration{}
				}
				clusterManager.Spec.RegistrationConfiguration.AutoApproveUsers = []string{"user1", "user2"}
				_, err = operatorClient.OperatorV1().ClusterManagers().Update(context.Background(), clusterManager, metav1.UpdateOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			gomega.Eventually(func() error {
				actual, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(), hubRegistrationDeployment, metav1.GetOptions{})
				if err != nil {
					return err
				}
				gomega.Expect(len(actual.Spec.Template.Spec.Containers)).Should(gomega.Equal(1))
				for _, arg := range actual.Spec.Template.Spec.Containers[0].Args {
					if arg == "--cluster-auto-approval-users=user1,user2" {
						return nil
					}
				}
				return fmt.Errorf("do not find the cluster-auto-approval-users args, got %v", actual.Spec.Template.Spec.Containers[0].Args)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
		})

	})

	ginkgo.Context("Cluster manager statuses", func() {
		ginkgo.It("should have correct degraded conditions", func() {
			gomega.Eventually(func() error {
				if _, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(
					context.Background(), hubRegistrationDeployment, metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// The cluster manager should be unavailable at first
			util.AssertClusterManagerCondition(clusterManagerName, operatorClient,
				"HubRegistrationDegraded", "UnavailableRegistrationPod", metav1.ConditionTrue)
			util.AssertClusterManagerCondition(clusterManagerName, operatorClient,
				"Progressing", "ClusterManagerDeploymentRolling", metav1.ConditionTrue)

			// Update replica of deployment
			updateDeploymentsStatusSuccess(kubeClient, hubNamespace,
				hubRegistrationDeployment, hubPlacementDeployment, hubRegistrationWebhookDeployment,
				hubWorkWebhookDeployment, hubWorkControllerDeployment, hubAddonManagerDeployment)

			// The cluster manager should be functional at last
			util.AssertClusterManagerCondition(clusterManagerName, operatorClient,
				"HubRegistrationDegraded", "RegistrationFunctional", metav1.ConditionFalse)
			util.AssertClusterManagerCondition(clusterManagerName, operatorClient,
				"Progressing", "ClusterManagerUpToDate", metav1.ConditionFalse)
		})
	})

	ginkgo.Context("Serving cert rotation", func() {
		ginkgo.It("should rotate both serving cert and signing cert before they become expired", func() {
			secretNames := []string{"signer-secret", "registration-webhook-serving-cert", "work-webhook-serving-cert"}
			// wait until all secrets and configmap are in place
			gomega.Eventually(func() error {
				for _, name := range secretNames {
					if _, err := kubeClient.CoreV1().Secrets(hubNamespace).Get(context.Background(), name, metav1.GetOptions{}); err != nil {
						return err
					}
				}
				if _, err := kubeClient.CoreV1().ConfigMaps(hubNamespace).Get(context.Background(), "ca-bundle-configmap", metav1.GetOptions{}); err != nil {
					return err
				}
				return nil
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			// both serving cert and signing cert should always be valid
			gomega.Consistently(func() error {
				configmap, err := kubeClient.CoreV1().ConfigMaps(hubNamespace).Get(context.Background(), "ca-bundle-configmap", metav1.GetOptions{})
				if err != nil {
					return err
				}
				for _, name := range []string{"signer-secret", "registration-webhook-serving-cert", "work-webhook-serving-cert"} {
					secret, err := kubeClient.CoreV1().Secrets(hubNamespace).Get(context.Background(), name, metav1.GetOptions{})
					if err != nil {
						return err
					}

					certificates, err := cert.ParseCertsPEM(secret.Data["tls.crt"])
					if err != nil {
						return err
					}
					if len(certificates) == 0 {
						return fmt.Errorf("certificates length equals to 0")
					}

					now := time.Now()
					certificate := certificates[0]
					if now.After(certificate.NotAfter) {
						return fmt.Errorf("certificate after NotAfter")
					}
					if now.Before(certificate.NotBefore) {
						return fmt.Errorf("certificate before NotBefore")
					}

					if name == "signer-secret" {
						continue
					}

					// ensure signing cert of serving certs in the ca bundle configmap
					caCerts, err := cert.ParseCertsPEM([]byte(configmap.Data["ca-bundle.crt"]))
					if err != nil {
						return err
					}

					found := false
					for _, caCert := range caCerts {
						if certificate.Issuer.CommonName != caCert.Subject.CommonName {
							continue
						}
						if now.After(caCert.NotAfter) {
							return fmt.Errorf("certificate after NotAfter")
						}
						if now.Before(caCert.NotBefore) {
							return fmt.Errorf("certificate before NotBefore")
						}
						found = true
						break
					}
					if !found {
						return fmt.Errorf("not found")
					}
				}
				return nil
			}, eventuallyTimeout*3, eventuallyInterval*3).Should(gomega.BeNil())
		})
	})

	ginkgo.Context("Cluster manager feature gates", func() {
		ginkgo.It("default should be set correctly", func() {
			// remove cluster manager configuration
			gomega.Eventually(func() error {
				clusterManager, err := operatorClient.OperatorV1().ClusterManagers().Get(context.Background(), clusterManagerName, metav1.GetOptions{})
				if err != nil {
					return err
				}

				// Check addon manager enabled mode
				clusterManager.Spec.RegistrationConfiguration = nil
				_, err = operatorClient.OperatorV1().ClusterManagers().Update(context.Background(), clusterManager, metav1.UpdateOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())
			gomega.Eventually(func() error {
				actual, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(),
					hubRegistrationDeployment, metav1.GetOptions{})
				if err != nil {
					return err
				}
				for _, arg := range actual.Spec.Template.Spec.Containers[0].Args {
					if arg == "--feature-gates=DefaultClusterSet=true" {
						return nil
					}
				}
				return fmt.Errorf("do not find the --feature-gates=DefaultClusterSet=true args, got %v", actual.Spec.Template.Spec.Containers[0].Args)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeNil())

			util.AssertClusterManagerCondition(clusterManagerName, operatorClient,
				helpers.FeatureGatesTypeValid, helpers.FeatureGatesReasonAllValid, metav1.ConditionTrue)

			workDeployment, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(),
				hubWorkWebhookDeployment, metav1.GetOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(workDeployment.Spec.Template.Spec.Containers[0].Args).Should(
				gomega.ContainElement("--feature-gates=NilExecutorValidating=true"))
			gomega.Expect(workDeployment.Spec.Template.Spec.Containers[0].Args).Should(
				gomega.ContainElement("--feature-gates=ManifestWorkReplicaSet=true"))

			workHubControllerDeployment, err := kubeClient.AppsV1().Deployments(hubNamespace).Get(context.Background(),
				hubWorkControllerDeployment, metav1.GetOptions{})
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
			gomega.Expect(workHubControllerDeployment.Spec.Template.Spec.Containers[0].Args).Should(
				gomega.ContainElement("manager"))
		})
	})
})
