package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apwide/k8s-monitor/pkg/golive"
	"gopkg.in/yaml.v3"
	"io"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"
)

type GoliveDataSender interface {
	PostEnvironmentInformation(ctx context.Context, info golive.PostEnvironmentInformationJSONRequestBody, reqEditors ...golive.RequestEditorFn) (*http.Response, error)
}

type GoliveLoggerSender struct {
	logger klog.Logger
	yaml   bool
}

func (g *GoliveLoggerSender) marshal(obj interface{}) ([]byte, error) {
	if g.yaml {
		return yaml.Marshal(obj)
	} else {
		return json.Marshal(obj)
	}
}

func (g *GoliveLoggerSender) PostEnvironmentInformation(ctx context.Context, info golive.PostEnvironmentInformationJSONRequestBody, reqEditors ...golive.RequestEditorFn) (*http.Response, error) {
	payload, err := g.marshal(info)
	if err != nil {
		g.logger.Error(err, "Not able to compite Golive Data Payload")
	} else {
		g.logger.Info("Golive Captured Data", "payload", string(payload))
	}
	return &http.Response{
		StatusCode: 200,
	}, nil
}

func NewHandler(ctx context.Context, clientSet *kubernetes.Clientset, listener Listener, golive GoliveDataSender, cfg Config) *Handler {
	logger := klog.FromContext(ctx).WithValues(
		"handler", listener.Id,
	)
	selectors := make([]ResourceSelector, len(listener.Selectors))
	for i, config := range listener.Selectors {
		labelSelector, err := labels.ValidatedSelectorFromSet(config.Labels)
		if err != nil {
			panic(err)
		}
		if config.LabelQuery != "" {
			requirements, err := labels.ParseToRequirements(config.LabelQuery)
			if err != nil {
				panic(err)
			}
			// TODO why labelSelector.Add(requirements) doesn't work ?
			for _, requirement := range requirements {
				labelSelector = labelSelector.Add(requirement)
			}
		}
		selectors[i] = ResourceSelector{config, labelSelector}
	}
	return &Handler{
		listener:              &listener,
		statusMapping:         cfg.StatusMapping,
		golive:                golive,
		ctx:                   ctx,
		logger:                logger,
		selectors:             selectors,
		clientSet:             clientSet,
		environmentAttributes: listener.FixedAttributes(),
	}
}

func (w *Handlers) getHandlerFor(pod *corev1.Pod) *Handler {
	index := slices.IndexFunc(w.handlers, func(w *Handler) bool {
		return w.match(pod)
	})
	if index > -1 {
		return w.handlers[index]
	} else {
		return nil
	}
}

func (w *Handlers) isListening(pod *corev1.Pod) bool {
	return w.getHandlerFor(pod) != nil
}

type ResourceSelector struct {
	config        Selector
	labelSelector labels.Selector
}

func (rs *ResourceSelector) match(pod *corev1.Pod) bool {
	logger := withPodValues(pod)
	if rs.config.Namespace != "" && pod.Namespace != rs.config.Namespace {
		logger.V(4).Info("Excluded by namespace")
		return false
	}
	if rs.config.Name != "" && pod.Name != rs.config.Name {
		logger.V(4).Info("Excluded by name")
		return false
	}
	logger = logger.WithValues("podLabels", pod.GetLabels(), "selectorLabels", rs.labelSelector)
	labelsMatch := rs.labelSelector.Matches(labels.Set(pod.GetLabels()))
	logger.V(4).Info(fmt.Sprintf("Label matched: %t", labelsMatch))
	return labelsMatch
}

type Handlers struct {
	handlers []*Handler
}

type Handler struct {
	listener              *Listener
	statusMapping         *StatusMapping
	ctx                   context.Context
	logger                klog.Logger
	clientSet             *kubernetes.Clientset
	golive                GoliveDataSender
	selectors             []ResourceSelector
	environmentAttributes map[string]string
}

func match(pod *corev1.Pod) func(s ResourceSelector) bool {
	return func(s ResourceSelector) bool {
		return s.match(pod)
	}
}

func (w *Handler) match(pod *corev1.Pod) bool {
	matched := slices.ContainsFunc(w.selectors, match(pod))
	w.logger.V(2).Info(fmt.Sprintf("Handler matched: %t", matched),
		"handler", w.listener.Id,
		"namespace", pod.GetNamespace(),
		"pod", pod.GetName(),
	)
	return matched
}

func (w *Handler) mapStatus(status EnvironmentStatus) *golive.NamedReference {
	var mappedStatus *NamedReference
	switch status {
	case Up:
		mappedStatus = &w.statusMapping.Up
	case Down:
		mappedStatus = &w.statusMapping.Down
	case Deploy:
		mappedStatus = &w.statusMapping.Deploy
	case Failed:
		mappedStatus = &w.statusMapping.Failed
	case Unknown:
	default:
	}
	if mappedStatus == nil {
		return nil
	}
	return &golive.NamedReference{Id: mappedStatus.Id, Name: mappedStatus.Name}
}

func (w *Handler) Handle(resource *MetaResource) {
	logger := w.loggerFor(resource)

	appRef, err := w.getApplicationReference(resource)
	if err != nil {
		logger.Error(err, "Unable to get application")
		runtime.HandleError(err)
		return
	}
	logger.V(4).Info("Application Found", "source", appRef.Source, "value", appRef.Value)

	catRef, err := w.getCategoryReference(resource)
	if err != nil {
		logger.Error(err, "Unable to get category")
		runtime.HandleError(err)
		return
	}
	logger.V(4).Info("Category Found", "source", catRef.Source, "value", catRef.Value)

	envName, err := w.getEnvironmentName(resource, appRef.Value, catRef.Value)
	if err != nil {
		logger.Error(err, "Unable to get environment name")
		runtime.HandleError(err)
		return
	}
	logger.V(4).Info("Environment Name Found", "source", envName.Source, "value", envName.Value)

	// TODO monitoring status => different task ? Check logic of k8s/prometheus to detect if up/down/ready (lifecycle)
	// TODO url from ingress (host) + path ?
	//url := ""
	attributes := w.getAttributes(resource)
	deploymentAttributes := map[string]string{}
	//buildNumber := ""

	versionName, err := w.getVersionName(resource)
	if err != nil {
		logger.Error(err, "Unable to get version name")
		runtime.HandleError(err)
		return
	}
	logger.V(4).Info("Version Name Found", "source", versionName.Source, "value", versionName.Value)

	var status *golive.NamedReference
	if w.statusMapping != nil {
		rscStatus, err := MetaStatus(resource)
		if err != nil {
			logger.Error(err, "Unable to get status from resource, will be ignored")
			runtime.HandleError(err)
		}
		status = w.mapStatus(rscStatus)
	} else {
		// TODO load status from Golive and try to identify status
		logger.V(4).Info("Status ignored due to missing status mapping configuration")
	}

	// TODO validate env info (fail if not possible to have valid selector)
	environmentInfo := golive.PostEnvironmentInformationJSONRequestBody{
		EnvironmentSelector: &golive.EnvironmentInfoSelector{
			Category: &golive.CreatableNamedReference{
				Id:         catRef.Value.Id,
				Name:       catRef.Value.Name,
				AutoCreate: &w.listener.AutoCreate,
			},
			Application: &golive.CreatableNamedReference{
				Id:         appRef.Value.Id,
				Name:       appRef.Value.Name,
				AutoCreate: &w.listener.AutoCreate,
			},
			Environment: &golive.CreatableNamedReference{
				//Id:         &envId,
				Name:       &envName.Value,
				AutoCreate: &w.listener.AutoCreate,
			},
		},
		Environment: golive.EnvironmentInfo{
			Name: &envName.Value, // if autocreate, do not provide value here
			//Url: => ingress ?
			Attributes: &attributes,
		},
		Deployment: &golive.DeploymentInfo{
			Attributes:  &deploymentAttributes,
			VersionName: &versionName.Value,
			//BuildNumber:  &buildNumber,
			//DeployedDate: &deployedDate, // added after check in cache
		},
		Status: status,
	}

	updated := goliveCache.setIfOutdated(environmentInfo.EnvironmentSelector, environmentInfo)
	if !updated {
		logger.V(4).Info("Golive should be up-to-date")
		return
	}

	// Check if deployment exists ?
	if environmentInfo.Deployment != nil {
		// TODO how to get Deployed Date
		// resource.getDeployedDate()
		deployedDate := time.Now().Format(time.RFC3339)
		if deployedDate == "" {
			deployedDate = time.Now().Format(time.RFC3339)
		}
		environmentInfo.Deployment.DeployedDate = &deployedDate
	}

	envInfo, err := w.golive.PostEnvironmentInformation(w.ctx, environmentInfo)

	if err != nil {
		logger.Error(err, "Error on pushing data to Golive")
	} else if envInfo.StatusCode < 200 || envInfo.StatusCode >= 400 {
		goliveCache.delete(environmentInfo.EnvironmentSelector)
		body, _ := io.ReadAll(envInfo.Body)
		err = fmt.Errorf("golive replied with %d and body: %s", envInfo.StatusCode, string(body))
		logger.Error(err, "Golive not updated")
	}
}

const (
	GolivePrefix          = "golive.apwide.net/"
	MaxAttributeValueSize = 255
	SuccessSynced         = "Synced"
	MessageSynced         = "Pod synced successfully"
	controllerAgentName   = "golive-controller"
	AppKey                = GolivePrefix + "app"
	CatKey                = GolivePrefix + "cat"
	VersionKey            = GolivePrefix + "version"
	NameKey               = GolivePrefix + "name"
)

var (
	goliveCache = newCache()
)

func isDefaultKey(key string) bool {
	return key == AppKey || key == CatKey || key == VersionKey || key == NameKey
}

func (w *Handler) getAttributes(resource *MetaResource) map[string]string {
	attributes := make(map[string]string)
	maps.Copy(attributes, w.environmentAttributes)
	for _, attribute := range w.listener.EnvironmentAttributes {
		if attribute.FromPath != "" {
			value, err := resource.GetJsonPath(attribute.FromPath)
			if err != nil {
				w.logger.Error(err, "Unable to extract attribute value using jsonPath")
			}
			if value != "" {
				attributes[attribute.Name] = value
			}
		}
	}
	for key, value := range resource.GetAnnotations() {
		if strings.HasPrefix(key, GolivePrefix) && !isDefaultKey(key) && value != "" {
			attribute := strings.TrimPrefix(key, GolivePrefix)
			attributes[attribute] = value
		}
	}

	for key, value := range attributes {
		attributes[key] = truncate(value, MaxAttributeValueSize)
	}
	return attributes
}

type ResourceValue[T any] struct {
	Source string
	Value  T
}

type NameReference struct {
	Id   *int32
	Name *string
}

func (r NameReference) String() string {
	return fmt.Sprintf("Id:%d,Name:%s", r.Id, *r.Name)
}

func (w *Handler) getNameReference(resource *MetaResource, config *CombinedReferenceSource, defaultKey string) (*ResourceValue[string], error) {
	logger := w.loggerFor(resource)
	var name string
	var err error
	if config.Value != "" {
		logger.V(4).Info("Reference value read from config")
		return &ResourceValue[string]{"Config", config.Value}, nil
	}
	// should read from namespace
	if config.Namespace {
		var ns *corev1.Namespace
		if ns, err = w.clientSet.CoreV1().Namespaces().Get(w.ctx, resource.GetNamespace(), metav1.GetOptions{}); err != nil {
			return nil, err
		}
		if config.Label != "" {
			if name = ns.Labels[config.Label]; name == "" {
				return nil, fmt.Errorf("label %q not found on on %q (%q)", config.Label, ns.Name, ns.Kind)
			} else {
				return &ResourceValue[string]{"Namespace Label", name}, nil
			}
		}
		if config.Annotation != "" {
			if name = ns.Annotations[config.Annotation]; name == "" {
				return nil, fmt.Errorf("annotation %q not found on %q (%q)", config.Annotation, ns.Name, ns.Kind)
			} else {
				return &ResourceValue[string]{"Namespace Annotation", name}, nil
			}
		}
		if name = ns.Labels[defaultKey]; name != "" {
			return &ResourceValue[string]{"Namespace Default Label", name}, nil
		}
		if name = ns.Annotations[defaultKey]; name != "" {
			return &ResourceValue[string]{"Namespace Default Annotation", name}, nil
		}
		return &ResourceValue[string]{"Namespace Name", ns.Name}, nil
	}

	if config.Label != "" {
		if name = resource.GetLabel(config.Label); name == "" {
			return nil, fmt.Errorf("label %q not found on %q (%q)", config.Label, resource.GetName(), resource.GetKind())
		} else {
			return &ResourceValue[string]{"Resource Label", name}, nil
		}
	}
	if config.Annotation != "" {
		if name = resource.GetAnnotation(config.Annotation); name == "" {
			return nil, fmt.Errorf("annotation %q not found on %q (%q)", config.Annotation, resource.GetName(), resource.GetKind())
		} else {
			return &ResourceValue[string]{"Resource Annotation", name}, nil
		}
	}
	if name = resource.GetLabels()[defaultKey]; name != "" {
		return &ResourceValue[string]{"Resource Default Label", name}, nil
	}
	if name = resource.GetAnnotations()[defaultKey]; name != "" {
		return &ResourceValue[string]{"Resource Default Annotation", name}, nil
	}
	return &ResourceValue[string]{"None", ""}, nil
}

func (w *Handler) getEnvironmentName(resource *MetaResource, app NameReference, cat NameReference) (value *ResourceValue[string], err error) {
	if value, err = w.getNameReference(resource, &w.listener.Name, NameKey); err != nil {
		return
	}
	if value.Value == "" {
		value = &ResourceValue[string]{"Resource Name", resource.GetName()}
	}
	if w.listener.Name.Template != "" {
		ctx := make(map[string]interface{})
		ctx["Value"] = value.Value
		ctx["App"] = app
		ctx["Cat"] = cat
		if value.Value, err = renderTemplate(w.listener.Name.Template, resource, ctx); err != nil {
			return
		}
	}
	return
}

func (w *Handler) getVersionName(resource *MetaResource) (value *ResourceValue[string], err error) {
	value, err = w.getNameReference(resource, &w.listener.Version.CombinedReferenceSource, VersionKey)
	if value.Value == "" && err == nil {
		var image *dockerImage
		if image, err = parseContainerImage(resource.GetPodTemplate().Spec.Containers[0].Image); image != nil {
			value = &ResourceValue[string]{"Image Tag", image.tag}
		}
	}
	if w.listener.Version.Template != "" {
		ctx := make(map[string]interface{})
		ctx["Value"] = value.Value
		if value.Value, err = renderTemplate(w.listener.Version.Template, resource, ctx); err != nil {
			return
		}
	}
	return
}

func (w *Handler) getApplicationReference(resource *MetaResource) (*ResourceValue[NameReference], error) {
	value, err := w.getNameReference(resource, &w.listener.Application, AppKey)
	if err != nil {
		return nil, err
	}
	if value.Value == "" && len(resource.GetPodTemplate().Spec.Containers) > 0 {
		image, err := parseContainerImage(resource.GetPodTemplate().Spec.Containers[0].Image)
		if err == nil {
			value = &ResourceValue[string]{"Image name", image.image}
		}
	}
	if value.Value == "" {
		return nil, fmt.Errorf("unable to extract application reference from %q (%q) in namespace %s", resource.GetName(), resource.GetKind(), resource.GetNamespace())
	}
	if w.listener.Application.Template != "" {
		ctx := make(map[string]interface{})
		ctx["Value"] = value.Value
		if value.Value, err = renderTemplate(w.listener.Application.Template, resource, ctx); err != nil {
			return nil, err
		}
	}
	return &ResourceValue[NameReference]{
		value.Source,
		NameReference{
			Name: &value.Value,
		},
	}, nil
}

func (w *Handler) getCategoryReference(resource *MetaResource) (*ResourceValue[NameReference], error) {
	value, err := w.getNameReference(resource, &w.listener.Category, CatKey)
	if err != nil {
		return nil, fmt.Errorf("unable to extract category reference from %q (%q) in namespace %s", resource.GetName(), resource.GetKind(), resource.GetNamespace())
	}
	if w.listener.Category.Template != "" {
		ctx := make(map[string]interface{})
		ctx["Value"] = value.Value
		if value.Value, err = renderTemplate(w.listener.Category.Template, resource, ctx); err != nil {
			return nil, err
		}
	}
	return &ResourceValue[NameReference]{
		value.Source,
		NameReference{
			Name: &value.Value,
		},
	}, nil
}

func (w *Handler) loggerFor(resource *MetaResource) klog.Logger {
	return w.logger.WithValues(
		"namespace", resource.GetNamespace(),
		"kind", resource.Type.GetKind(),
		"name", resource.GetName(),
	)
}

func withPodValues(pod *corev1.Pod) klog.Logger {
	return klog.FromContext(context.Background()).WithValues("namespace", pod.GetNamespace(),
		"name", pod.GetName())
}
