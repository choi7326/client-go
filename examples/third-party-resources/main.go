package main

import (
	"flag"
	"fmt"

	"github.com/FlorianOtel/client-go/kubernetes"
	"github.com/FlorianOtel/client-go/pkg/api"
	"github.com/FlorianOtel/client-go/pkg/api/errors"
	"github.com/FlorianOtel/client-go/pkg/api/v1"
	"github.com/FlorianOtel/client-go/pkg/apis/extensions/v1beta1"
	metav1 "github.com/FlorianOtel/client-go/pkg/apis/meta/v1"
	"github.com/FlorianOtel/client-go/pkg/runtime"
	"github.com/FlorianOtel/client-go/pkg/runtime/schema"
	"github.com/FlorianOtel/client-go/pkg/runtime/serializer"
	"github.com/FlorianOtel/client-go/rest"
	"github.com/FlorianOtel/client-go/tools/clientcmd"

	// Only required to authenticate against GKE clusters
	_ "github.com/FlorianOtel/client-go/plugin/pkg/client/auth/gcp"
)

var (
	config *rest.Config
)

func main() {
	kubeconfig := flag.String("kubeconfig", "", "Path to a kube config. Only required if out-of-cluster.")
	flag.Parse()

	// Create the client config. Use kubeconfig if given, otherwise assume in-cluster.
	config, err := buildConfig(*kubeconfig)
	if err != nil {
		panic(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	// initialize third party resource if it does not exist
	tpr, err := clientset.Extensions().ThirdPartyResources().Get("example.k8s.io", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			tpr := &v1beta1.ThirdPartyResource{
				ObjectMeta: v1.ObjectMeta{
					Name: "example.k8s.io",
				},
				Versions: []v1beta1.APIVersion{
					{Name: "v1"},
				},
				Description: "An Example ThirdPartyResource",
			}

			result, err := clientset.Extensions().ThirdPartyResources().Create(tpr)
			if err != nil {
				panic(err)
			}
			fmt.Printf("CREATED: %#v\nFROM: %#v\n", result, tpr)
		} else {
			panic(err)
		}
	} else {
		fmt.Printf("SKIPPING: already exists %#v\n", tpr)
	}

	// make a new config for our extension's API group, using the first config as a baseline
	var tprconfig *rest.Config
	tprconfig = config
	configureClient(tprconfig)

	tprclient, err := rest.RESTClientFor(tprconfig)
	if err != nil {
		panic(err)
	}

	var example Example

	err = tprclient.Get().
		Resource("examples").
		Namespace(api.NamespaceDefault).
		Name("example1").
		Do().Into(&example)

	if err != nil {
		if errors.IsNotFound(err) {
			// Create an instance of our TPR
			example := &Example{
				Metadata: api.ObjectMeta{
					Name: "example1",
				},
				Spec: ExampleSpec{
					Foo: "hello",
					Bar: true,
				},
			}

			var result Example
			err = tprclient.Post().
				Resource("examples").
				Namespace(api.NamespaceDefault).
				Body(example).
				Do().Into(&result)

			if err != nil {
				panic(err)
			}
			fmt.Printf("CREATED: %#v\n", result)
		} else {
			panic(err)
		}
	} else {
		fmt.Printf("GET: %#v\n", example)
	}

	// Fetch a list of our TPRs
	exampleList := ExampleList{}
	err = tprclient.Get().Resource("examples").Do().Into(&exampleList)
	if err != nil {
		panic(err)
	}
	fmt.Printf("LIST: %#v\n", exampleList)
}

func buildConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func configureClient(config *rest.Config) {
	groupversion := schema.GroupVersion{
		Group:   "k8s.io",
		Version: "v1",
	}

	config.GroupVersion = &groupversion
	config.APIPath = "/apis"
	config.ContentType = runtime.ContentTypeJSON
	config.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}

	schemeBuilder := runtime.NewSchemeBuilder(
		func(scheme *runtime.Scheme) error {
			scheme.AddKnownTypes(
				groupversion,
				&Example{},
				&ExampleList{},
				&api.ListOptions{},
				&api.DeleteOptions{},
			)
			return nil
		})
	schemeBuilder.AddToScheme(api.Scheme)
}
