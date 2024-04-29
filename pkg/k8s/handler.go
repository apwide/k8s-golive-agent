package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/apwide/k8s-monitor/pkg/cache"
	"github.com/apwide/k8s-monitor/pkg/golive"
	"gopkg.in/yaml.v3"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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

func NewHandler(ctx context.Context, clientSet *kubernetes.Clientset, listener Listener, golive GoliveDataSender, cfg Config, cache *cache.GoliveCache) *Handler {
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
		cache:                 cache,
	}
}

func (w *Handlers) getHandlersFor(pod *corev1.Pod) []*Handler {
	matchedHandlers := make([]*Handler, 0)
	for _, handler := range w.handlers {
		if handler.match(pod) {
			matchedHandlers = append(matchedHandlers, handler)
		}
	}
	return matchedHandlers
}

func (w *Handlers) isListening(pod *corev1.Pod) bool {
	return len(w.getHandlersFor(pod)) > 0
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
	cache                 *cache.GoliveCache
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
	if len(w.selectors) == 0 {
		return true
	}
	matched := slices.ContainsFunc(w.selectors, match(pod))
	w.logger.V(2).Info(fmt.Sprintf("Handler matched: %t", matched),
		"handler", w.listener.Id,
		"namespace", pod.GetNamespace(),
		"pod", pod.GetName(),
	)
	return matched
}

func (w *Handler) Handle(resource *MetaResource) {
	logger := w.loggerFor(resource)
	var (
		err        error
		appName    string
		catName    string
		envName    string
		envUrl     *string
		attributes map[string]string
		status     *golive.NamedReference
		deployment *golive.DeploymentInfo
	)

	if appName, err = w.getApplication(resource); err != nil {
		logger.Error(err, "Error on looking for application")
		return
	}
	if catName, err = w.getCategory(resource); err != nil {
		logger.Error(err, "Error on looking for category")
		return
	}
	if envName, err = w.getEnvironmentName(resource, NameReference{Name: &appName}, NameReference{Name: &catName}); err != nil {
		logger.Error(err, "Error on looking for environment name")
		return
	}
	if url, err := w.getEnvironmentUrl(resource, NameReference{Name: &appName}, NameReference{Name: &catName}); err != nil {
		logger.Error(err, "Error on looking for environment url")
		return
	} else if url != "" {
		envUrl = &url
	}
	if attributes, err = w.getEnvironmentAttributes(resource); err != nil {
		logger.Error(err, "Error on looking for environment attributes")
		return
	}
	if w.statusMapping != nil {
		if status, err = w.getStatus(resource); err != nil {
			logger.Error(err, "Error on looking for status, will be ignored")
		}
	} else {
		// TODO load status from Golive and try to identify status
		logger.V(4).Info("Status ignored due to missing status mapping configuration")
	}
	if !w.listener.Deployment.Ignore {
		if deployment, err = w.getDeployment(resource); err != nil {
			logger.Error(err, "Error on looking for deployment")
			return
		}
	} else {
		logger.V(4).Info("Version ignored by configuration")
	}

	environmentInfo := golive.PostEnvironmentInformationJSONRequestBody{
		EnvironmentSelector: &golive.EnvironmentInfoSelector{
			Category: &golive.CreatableNamedReference{
				Id:         nil,
				Name:       &catName,
				AutoCreate: &w.listener.AutoCreate,
			},
			Application: &golive.CreatableNamedReference{
				Id:         nil,
				Name:       &appName,
				AutoCreate: &w.listener.AutoCreate,
			},
			Environment: &golive.CreatableNamedReference{
				Id:         nil,
				Name:       &envName,
				AutoCreate: &w.listener.AutoCreate,
			},
		},
		Environment: golive.EnvironmentInfo{
			Name:       &envName, // if autocreate, do not provide value here
			Url:        envUrl,
			Attributes: &attributes,
		},
		Deployment: deployment,
		Status:     status,
	}

	updated := w.cache.SetIfOutdated(environmentInfo.EnvironmentSelector, environmentInfo)
	if !updated {
		logger.V(4).Info("Golive should be up-to-date")
		return
	}

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
		w.cache.Delete(environmentInfo.EnvironmentSelector)
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
	None                  = ""
	EnvKey                = GolivePrefix + "environment"
	EnvAttributePrefix    = "env." + GolivePrefix
	DeployAttributePrefix = "deploy." + GolivePrefix
	UrlKey                = GolivePrefix + "url"
)

func isDefaultKey(key string) bool {
	return key == AppKey || key == CatKey || key == VersionKey || key == EnvKey
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

func (w *Handler) getEnvironmentUrl(resource *MetaResource, app NameReference, cat NameReference) (string, error) {
	ctx := make(map[string]interface{})
	ctx["App"] = app
	ctx["Cat"] = cat
	if w.listener.Environment.Url != "" {
		return renderTemplate(w.listener.Environment.Url, resource, UrlKey, ctx)
	}
	defaultTemplates := []string{"{{ defaultLabel }}", "{{ defaultAnnotation }}", "{{ nsDefaultLabel }}", "{{ nsDefaultAnnotation }}"}
	for _, template := range defaultTemplates {
		if value, err := renderTemplate(template, resource, UrlKey, ctx); err != nil || value != "" {
			return value, err
		}
	}
	return "", nil
}

func (w *Handler) getEnvironmentName(resource *MetaResource, app NameReference, cat NameReference) (string, error) {
	ctx := make(map[string]interface{})
	ctx["App"] = app
	ctx["Cat"] = cat
	if w.listener.Environment.Name != "" {
		if value, err := renderTemplate(w.listener.Environment.Name, resource, EnvKey, ctx); err == nil && value == "" {
			return value, fmt.Errorf("provided template did not match anything")
		} else {
			return value, err
		}
	}
	defaultTemplates := []string{"{{ defaultLabel }}", "{{ defaultAnnotation }}", "{{ nsDefaultLabel }}", "{{ nsDefaultAnnotation }}", "{{ name }}"}
	for _, template := range defaultTemplates {
		if value, err := renderTemplate(template, resource, EnvKey, ctx); err != nil || value != "" {
			return value, err
		}
	}
	return "", fmt.Errorf("none of the default templates got a result")
}

func (w *Handler) getEnvironmentAttributes(resource *MetaResource) (map[string]string, error) {
	ctx := make(map[string]interface{})
	attributes := make(map[string]string)
	for _, attribute := range w.listener.Environment.Attributes {
		if value, err := renderTemplate(attribute.Value, resource, None, ctx); err != nil {
			return nil, err
		} else {
			attributes[attribute.Name] = value
		}
	}
	for key, value := range resource.GetAnnotations() {
		if strings.HasPrefix(key, EnvAttributePrefix) && !isDefaultKey(key) && value != "" {
			attribute := strings.TrimPrefix(key, EnvAttributePrefix)
			attributes[attribute] = value
		}
	}
	for key, value := range resource.ns.GetAnnotations() {
		if strings.HasPrefix(key, EnvAttributePrefix) && !isDefaultKey(key) && value != "" {
			attribute := strings.TrimPrefix(key, EnvAttributePrefix)
			attributes[attribute] = value
		}
	}
	for key, value := range attributes {
		attributes[key] = truncate(value, MaxAttributeValueSize)
	}
	return attributes, nil
}

func (w *Handler) getVersionName(resource *MetaResource) (string, error) {
	ctx := make(map[string]interface{})
	if w.listener.Deployment.VersionName != "" {
		return renderTemplate(w.listener.Deployment.VersionName, resource, VersionKey, ctx)
	}
	defaultTemplates := []string{"{{ defaultLabel }}", "{{ defaultAnnotation }}", "{{ mainImageTag }}"}
	for _, template := range defaultTemplates {
		if value, err := renderTemplate(template, resource, VersionKey, ctx); err != nil || value != "" {
			return value, err
		}
	}
	return "", nil
}

func (w *Handler) getApplication(resource *MetaResource) (string, error) {
	ctx := make(map[string]interface{})
	if w.listener.Application.Name != "" {
		if value, err := renderTemplate(w.listener.Application.Name, resource, AppKey, ctx); err == nil && value == "" {
			return value, fmt.Errorf("provided template did not match anything")
		} else {
			return value, err
		}
	}
	defaultTemplates := []string{"{{ defaultLabel }}", "{{ defaultAnnotation }}", "{{ mainImageName }}"}
	for _, template := range defaultTemplates {
		if value, err := renderTemplate(template, resource, AppKey, ctx); err != nil || value != "" {
			return value, err
		}
	}
	return "", fmt.Errorf("none of the default templates got a result")
}

func (w *Handler) getCategory(resource *MetaResource) (string, error) {
	ctx := make(map[string]interface{})
	if w.listener.Category.Name != "" {
		if value, err := renderTemplate(w.listener.Category.Name, resource, CatKey, ctx); err == nil && value == "" {
			return value, fmt.Errorf("provided template did not match anything")
		} else {
			return value, err
		}
	}
	defaultTemplates := []string{"{{ defaultLabel }}", "{{ defaultAnnotation }}", "{{ nsDefaultLabel }}", "{{ nsDefaultAnnotation }}", "{{ nsName }}"}
	for _, template := range defaultTemplates {
		if value, err := renderTemplate(template, resource, CatKey, ctx); err != nil || value != "" {
			return value, err
		}
	}
	return "", fmt.Errorf("none of the default templates got a result")
}

func (w *Handler) getStatus(resource *MetaResource) (*golive.NamedReference, error) {
	if rscStatus, err := MetaStatus(resource); err != nil {
		return nil, err
	} else {
		var mappedStatus *NamedReference
		switch rscStatus {
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
			return nil, nil
		}
		return &golive.NamedReference{
			Id:   mappedStatus.Id,
			Name: mappedStatus.Name,
		}, nil
	}
}

func (w *Handler) getDeployment(resource *MetaResource) (*golive.DeploymentInfo, error) {
	var (
		versionName          string
		deploymentAttributes map[string]string
		err                  error
	)
	if versionName, err = w.getVersionName(resource); err != nil {
		return nil, err
	}
	if deploymentAttributes, err = w.getDeploymentAttributes(resource); err != nil {
		return nil, err
	}
	return &golive.DeploymentInfo{
		Attributes:  &deploymentAttributes,
		VersionName: &versionName,
		//BuildNumber:  &buildNumber,
		//DeployedDate: &deployedDate, // added after check in cache
	}, nil
}

func (w *Handler) getDeploymentAttributes(resource *MetaResource) (map[string]string, error) {
	ctx := make(map[string]interface{})
	attributes := make(map[string]string)
	for _, attribute := range w.listener.Deployment.Attributes {
		if value, err := renderTemplate(attribute.Value, resource, None, ctx); err != nil {
			return nil, err
		} else {
			attributes[attribute.Name] = value
		}
	}
	for key, value := range resource.GetAnnotations() {
		if strings.HasPrefix(key, DeployAttributePrefix) && !isDefaultKey(key) && value != "" {
			attribute := strings.TrimPrefix(key, DeployAttributePrefix)
			attributes[attribute] = value
		}
	}
	for key, value := range resource.ns.GetAnnotations() {
		if strings.HasPrefix(key, DeployAttributePrefix) && !isDefaultKey(key) && value != "" {
			attribute := strings.TrimPrefix(key, DeployAttributePrefix)
			attributes[attribute] = value
		}
	}
	for key, value := range attributes {
		attributes[key] = truncate(value, MaxAttributeValueSize)
	}
	return attributes, nil
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
