package dclient

import (
	"errors"
	"fmt"
	"strings"

	openapiv2 "github.com/google/gnostic-models/openapiv2"
	kubeutils "github.com/kyverno/kyverno/pkg/utils/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// NewFakeClient ---testing utilities
func NewFakeClient(scheme *runtime.Scheme, gvrToListKind map[schema.GroupVersionResource]string, objects ...runtime.Object) (Interface, error) {
	unstructuredScheme := runtime.NewScheme()
	for gvk := range scheme.AllKnownTypes() {
		if unstructuredScheme.Recognizes(gvk) {
			continue
		}
		if strings.HasSuffix(gvk.Kind, "List") {
			unstructuredScheme.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
			continue
		}
		unstructuredScheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	}
	objects, err := convertObjectsToUnstructured(objects)
	if err != nil {
		panic(err)
	}
	for _, obj := range objects {
		gvk := obj.GetObjectKind().GroupVersionKind()
		if !unstructuredScheme.Recognizes(gvk) {
			unstructuredScheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		}
		gvk.Kind += "List"
		if !unstructuredScheme.Recognizes(gvk) {
			unstructuredScheme.AddKnownTypeWithName(gvk, &unstructured.UnstructuredList{})
		}
	}
	c := fake.NewSimpleDynamicClientWithCustomListKinds(unstructuredScheme, gvrToListKind, objects...)
	// the typed and dynamic client are initialized with similar resources
	kclient := kubefake.NewSimpleClientset(objects...)
	return &client{
		dyn:  c,
		kube: kclient,
	}, nil
}

func NewEmptyFakeClient() Interface {
	gvrToListKind := map[schema.GroupVersionResource]string{}
	objects := []runtime.Object{}
	scheme := runtime.NewScheme()
	kclient := kubefake.NewSimpleClientset(objects...)
	return &client{
		dyn:   fake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objects...),
		disco: NewFakeDiscoveryClient(nil),
		kube:  kclient,
	}
}

// NewFakeDiscoveryClient returns a fakediscovery client
func NewFakeDiscoveryClient(registeredResources []schema.GroupVersionResource) *fakeDiscoveryClient {
	// Load some-preregistered resources
	res := []schema.GroupVersionResource{
		{Version: "v1", Resource: "configmaps"},
		{Version: "v1", Resource: "endpoints"},
		{Version: "v1", Resource: "namespaces"},
		{Version: "v1", Resource: "resourcequotas"},
		{Version: "v1", Resource: "secrets"},
		{Version: "v1", Resource: "serviceaccounts"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
	}
	registeredResources = append(registeredResources, res...)
	return &fakeDiscoveryClient{registeredResources: registeredResources}
}

type fakeDiscoveryClient struct {
	registeredResources []schema.GroupVersionResource
}

func (c *fakeDiscoveryClient) getGVR(resource string) (schema.GroupVersionResource, error) {
	for _, gvr := range c.registeredResources {
		if gvr.Resource == resource {
			return gvr, nil
		}
	}
	return schema.GroupVersionResource{}, errors.New("not found")
}

func (c *fakeDiscoveryClient) GetGVKFromGVR(schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return schema.GroupVersionKind{}, nil
}

func (c *fakeDiscoveryClient) GetGVRFromGVK(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	resource := strings.ToLower(gvk.Kind) + "s"
	return c.getGVR(resource)
}

func (c *fakeDiscoveryClient) FindResources(group, version, kind, subresource string) (map[TopLevelApiDescription]metav1.APIResource, error) {
	r := strings.ToLower(kind) + "s"
	for _, resource := range c.registeredResources {
		if resource.Resource == r {
			return map[TopLevelApiDescription]metav1.APIResource{
				{
					GroupVersion: schema.GroupVersion{Group: resource.Group, Version: resource.Version},
					Kind:         kind,
					Resource:     r,
					SubResource:  subresource,
				}: {},
			}, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (c *fakeDiscoveryClient) OpenAPISchema() (*openapiv2.Document, error) {
	return nil, nil
}

func (c *fakeDiscoveryClient) CachedDiscoveryInterface() discovery.CachedDiscoveryInterface {
	return nil
}

func (c *fakeDiscoveryClient) OnChanged(callback func()) {
	// No-op for fake client
}

func convertObjectsToUnstructured(objs []runtime.Object) ([]runtime.Object, error) {
	ul := make([]runtime.Object, 0, len(objs))
	for _, obj := range objs {
		u, err := kubeutils.ObjToUnstructured(obj)
		if err != nil {
			return nil, err
		}
		ul = append(ul, u)
	}
	return ul, nil
}
