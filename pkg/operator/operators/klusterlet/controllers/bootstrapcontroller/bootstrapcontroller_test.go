package bootstrapcontroller

import (
	"context"
	cryptorand "crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	fakekube "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	certutil "k8s.io/client-go/util/cert"

	fakeoperatorclient "open-cluster-management.io/api/client/operator/clientset/versioned/fake"
	operatorinformers "open-cluster-management.io/api/client/operator/informers/externalversions"
	operatorapiv1 "open-cluster-management.io/api/operator/v1"

	"open-cluster-management.io/ocm/pkg/common/patcher"
	testingcommon "open-cluster-management.io/ocm/pkg/common/testing"
	"open-cluster-management.io/ocm/pkg/operator/helpers"
)

func TestSync(t *testing.T) {
	cases := []struct {
		name                    string
		queueKey                string
		klusterletInstallMode   operatorapiv1.InstallMode
		initRebootstrapping     bool
		objects                 []runtime.Object
		expectedRebootstrapping bool
		validateActions         func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name:    "the changed secret is not bootstrap secret",
			objects: []runtime.Object{},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Errorf("expected no actions happens, but got %#v", actions)
				}
			},
		},
		{
			name:     "client certificate expired",
			queueKey: "test/test",
			objects: []runtime.Object{
				newSecret("bootstrap-hub-kubeconfig", "test", newKubeConfig("https://10.0.118.47:6443", "")),
				newHubKubeConfigSecret("test", time.Now().Add(-60*time.Second).UTC()),
			},
			expectedRebootstrapping: true,
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Errorf("expected no actions happens, but got %#v", actions)
				}
			},
		},
		{
			name:     "the bootstrap is not started",
			queueKey: "test/test",
			objects:  []runtime.Object{newSecret("bootstrap-hub-kubeconfig", "test", newKubeConfig("https://10.0.118.47:6443", ""))},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Errorf("expected no actions happens, but got %#v", actions)
				}
			},
		},
		{
			name:     "the bootstrap secret is not changed",
			queueKey: "test/test",
			objects: []runtime.Object{
				newSecret("bootstrap-hub-kubeconfig", "test", newKubeConfig("https://10.0.118.47:6443", "")),
				newHubKubeConfigSecret("test", time.Now().Add(60*time.Second).UTC()),
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Errorf("expected no actions happens, but got %#v", actions)
				}
			},
		},
		{
			name:     "hub server url is changed",
			queueKey: "test/test",
			objects: []runtime.Object{
				newSecret("bootstrap-hub-kubeconfig", "test", newKubeConfig("https://10.0.118.48:6443", "")),
				newHubKubeConfigSecret("test", time.Now().Add(60*time.Second).UTC()),
			},
			expectedRebootstrapping: true,
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Errorf("expected no actions happens, but got %#v", actions)
				}
			},
		},
		{
			name:     "proxy url is changed",
			queueKey: "test/test",
			objects: []runtime.Object{
				newSecret("bootstrap-hub-kubeconfig", "test", newKubeConfig("https://10.0.118.48:6443", "https://10.0.118.10:3129")),
				newHubKubeConfigSecret("test", time.Now().Add(60*time.Second).UTC()),
			},
			expectedRebootstrapping: true,
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				if len(actions) != 0 {
					t.Errorf("expected no actions happens, but got %#v", actions)
				}
			},
		},
		{
			name:                  "wait for scaling down",
			queueKey:              "test/test",
			klusterletInstallMode: operatorapiv1.InstallModeSingleton,
			initRebootstrapping:   true,
			objects: []runtime.Object{
				newSecret("bootstrap-hub-kubeconfig", "test", newKubeConfig("https://10.0.118.48:6443", "")),
				newHubKubeConfigSecret("test", time.Now().Add(60*time.Second).UTC()),
				newDeploymentWithAvailableReplicas("test-agent", "test", 1),
			},
			expectedRebootstrapping: true,
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testingcommon.AssertGet(t, actions[0], "apps", "v1", "deployments")
			},
		},
		{
			name:                "rebootstrap is completed",
			queueKey:            "test/test",
			initRebootstrapping: true,
			objects: []runtime.Object{
				newSecret("bootstrap-hub-kubeconfig", "test", newKubeConfig("https://10.0.118.48:6443", "")),
				newHubKubeConfigSecret("test", time.Now().Add(60*time.Second).UTC()),
				newDeployment("test-registration-agent", "test"),
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testingcommon.AssertDelete(t, actions[1], "secrets", "test", "hub-kubeconfig-secret")
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeKubeClient := fakekube.NewSimpleClientset(c.objects...)
			klusterlet := newKlusterlet("test", "test", c.klusterletInstallMode)
			if c.initRebootstrapping {
				klusterlet.Status.Conditions = []metav1.Condition{
					{
						Type:   helpers.KlusterletRebootstrapProgressing,
						Status: metav1.ConditionTrue,
					},
				}
			}
			fakeOperatorClient := fakeoperatorclient.NewSimpleClientset(klusterlet)
			operatorInformers := operatorinformers.NewSharedInformerFactory(fakeOperatorClient, 5*time.Minute)
			operatorStore := operatorInformers.Operator().V1().Klusterlets().Informer().GetStore()
			if err := operatorStore.Add(klusterlet); err != nil {
				t.Fatal(err)
			}

			newOnTermInformer := func(name string) kubeinformers.SharedInformerFactory {
				return kubeinformers.NewSharedInformerFactoryWithOptions(fakeKubeClient, 5*time.Minute,
					kubeinformers.WithTweakListOptions(func(options *metav1.ListOptions) {
						options.FieldSelector = fields.OneTermEqualSelector("metadata.name", name).String()
					}))
			}

			secretInformers := map[string]corev1informers.SecretInformer{
				helpers.HubKubeConfig:             newOnTermInformer(helpers.HubKubeConfig).Core().V1().Secrets(),
				helpers.BootstrapHubKubeConfig:    newOnTermInformer(helpers.BootstrapHubKubeConfig).Core().V1().Secrets(),
				helpers.ExternalManagedKubeConfig: newOnTermInformer(helpers.ExternalManagedKubeConfig).Core().V1().Secrets(),
			}

			for _, o := range c.objects {
				switch object := o.(type) {
				case *corev1.Secret:
					switch object.Name {
					case helpers.HubKubeConfig:
						secretStore := secretInformers[helpers.HubKubeConfig].Informer().GetStore()
						if err := secretStore.Add(object); err != nil {
							t.Fatal(err)
						}
					case helpers.BootstrapHubKubeConfig:
						secretStore := secretInformers[helpers.BootstrapHubKubeConfig].Informer().GetStore()
						if err := secretStore.Add(object); err != nil {
							t.Fatal(err)
						}
					case helpers.ExternalManagedKubeConfig:
						secretStore := secretInformers[helpers.ExternalManagedKubeConfig].Informer().GetStore()
						if err := secretStore.Add(object); err != nil {
							t.Fatal(err)
						}
					}
				}
			}

			klusterletClient := fakeOperatorClient.OperatorV1().Klusterlets()
			klusterletPatcher := patcher.NewPatcher[*operatorapiv1.Klusterlet, operatorapiv1.KlusterletSpec, operatorapiv1.KlusterletStatus](klusterletClient)
			controller := &bootstrapController{
				kubeClient:       fakeKubeClient,
				klusterletClient: klusterletClient,
				klusterletLister: operatorInformers.Operator().V1().Klusterlets().Lister(),
				secretInformers:  secretInformers,
				patcher:          klusterletPatcher,
			}

			syncContext := testingcommon.NewFakeSyncContext(t, c.queueKey)
			if err := controller.sync(context.TODO(), syncContext); err != nil {
				t.Errorf("Expected no errors, but got %v", err)
			}

			c.validateActions(t, fakeKubeClient.Actions())

			klusterlet, err := fakeOperatorClient.OperatorV1().Klusterlets().Get(context.Background(), klusterlet.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("Expected no errors, but got %v", err)
			}
			rebootstrapping := meta.IsStatusConditionTrue(klusterlet.Status.Conditions, helpers.KlusterletRebootstrapProgressing)
			if c.expectedRebootstrapping != rebootstrapping {
				t.Errorf("Expected rebootstrapping is %v, but got %v", c.expectedRebootstrapping, rebootstrapping)
			}
		})
	}
}

func TestBootstrapSecretQueueKeyFunc(t *testing.T) {
	cases := []struct {
		name        string
		object      runtime.Object
		klusterlet  *operatorapiv1.Klusterlet
		expectedKey []string
	}{
		{
			name:        "key by bootstrap secret",
			object:      newSecret("bootstrap-hub-kubeconfig", "test", []byte{}),
			klusterlet:  newKlusterlet("testklusterlet", "test", ""),
			expectedKey: []string{"test/testklusterlet"},
		},
		{
			name:        "key by wrong secret",
			object:      newSecret("dummy", "test", []byte{}),
			klusterlet:  newKlusterlet("testklusterlet", "test", ""),
			expectedKey: []string{},
		},
		{
			name:        "key by klusterlet with empty namespace",
			object:      newSecret("bootstrap-hub-kubeconfig", "open-cluster-management-agent", []byte{}),
			klusterlet:  newKlusterlet("testklusterlet", "", ""),
			expectedKey: []string{"open-cluster-management-agent/testklusterlet"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeOperatorClient := fakeoperatorclient.NewSimpleClientset(c.klusterlet)
			operatorInformers := operatorinformers.NewSharedInformerFactory(fakeOperatorClient, 5*time.Minute)
			store := operatorInformers.Operator().V1().Klusterlets().Informer().GetStore()
			if err := store.Add(c.klusterlet); err != nil {
				t.Fatal(err)
			}
			keyFunc := bootstrapSecretQueueKeyFunc(operatorInformers.Operator().V1().Klusterlets().Lister())
			actualKey := keyFunc(c.object)
			if !equality.Semantic.DeepEqual(actualKey, c.expectedKey) {
				t.Errorf("Queued key is not correct: actual %s, expected %s", actualKey, c.expectedKey)
			}
		})
	}
}

func newKlusterlet(name, namespace string, installMode operatorapiv1.InstallMode) *operatorapiv1.Klusterlet {
	return &operatorapiv1.Klusterlet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: operatorapiv1.KlusterletSpec{
			Namespace: namespace,
			DeployOption: operatorapiv1.KlusterletDeployOption{
				Mode: installMode,
			},
		},
	}
}

func newSecret(name, namespace string, kubeConfig []byte) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{},
	}
	secret.Data["kubeconfig"] = kubeConfig
	return secret
}

func newKubeConfig(host, proxyURL string) []byte {
	configData, _ := runtime.Encode(clientcmdlatest.Codec, &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{"default-cluster": {
			Server:                host,
			InsecureSkipTLSVerify: true,
			ProxyURL:              proxyURL,
		}},
		Contexts: map[string]*clientcmdapi.Context{"default-context": {
			Cluster: "default-cluster",
		}},
		CurrentContext: "default-context",
	})
	return configData
}

func newHubKubeConfigSecret(namespace string, notAfter time.Time) *corev1.Secret {
	caKey, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	caCert, err := certutil.NewSelfSignedCACert(certutil.Config{CommonName: "open-cluster-management.io"}, caKey)
	if err != nil {
		panic(err)
	}

	key, err := rsa.GenerateKey(cryptorand.Reader, 2048)
	if err != nil {
		panic(err)
	}

	certDERBytes, err := x509.CreateCertificate(
		cryptorand.Reader,
		&x509.Certificate{
			Subject: pkix.Name{
				CommonName: "test",
			},
			SerialNumber: big.NewInt(1),
			NotBefore:    caCert.NotBefore,
			NotAfter:     notAfter,
			KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
		caCert,
		key.Public(),
		caKey,
	)
	if err != nil {
		panic(err)
	}

	cert, err := x509.ParseCertificate(certDERBytes)
	if err != nil {
		panic(err)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "hub-kubeconfig-secret",
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"kubeconfig": newKubeConfig("https://10.0.118.47:6443", ""),
			"tls.crt": pem.EncodeToMemory(&pem.Block{
				Type:  certutil.CertificateBlockType,
				Bytes: cert.Raw,
			}),
		},
	}
}

func newDeployment(name, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{},
	}
}

func newDeploymentWithAvailableReplicas(name, namespace string, availableReplicas int32) *appsv1.Deployment {
	deploy := newDeployment(name, namespace)
	deploy.Status.AvailableReplicas = availableReplicas
	return deploy
}
