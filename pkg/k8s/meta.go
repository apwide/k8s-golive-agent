package k8s

import (
	"fmt"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"slices"
	"strconv"
	"time"
)

type EnvironmentStatus int

const (
	RevisionAnnotation = "deployment.kubernetes.io/revision"
	TimedOutReason     = "ProgressDeadlineExceeded"
)

const (
	Unknown EnvironmentStatus = iota
	Down
	Deploy
	Failed
	Up
)

// launch tracking function on Deployment/Daemon when deployment detected

type Deployment struct {
	*appsv1.Deployment `json:",inline"`
}
type StatefulSet struct {
	*appsv1.StatefulSet `json:",inline"`
}
type DaemonSet struct {
	*appsv1.DaemonSet `json:",inline"`
}
type CronJob struct {
	*batchv1.CronJob `json:",inline"`
}

type MetaResource struct {
	ns *corev1.Namespace
	metav1.Object
	metav1.Type
	Listenable `json:",inline"`
}

type Listenable interface {
	GetOriginal() interface{}
	GetPodTemplate() corev1.PodTemplateSpec
	getDeployedDate() string
}

type PodResource struct {
	*corev1.Pod
}

func (d Deployment) GetOriginal() interface{} {
	return d.Deployment
}

func (d Deployment) GetPodTemplate() corev1.PodTemplateSpec {
	return d.Spec.Template
}

func (d Deployment) getDeployedDate() string {
	readyIdx := slices.IndexFunc(d.Status.Conditions, func(c appsv1.DeploymentCondition) bool { return c.Type == appsv1.DeploymentAvailable })
	if readyIdx > -1 {
		return d.Status.Conditions[readyIdx].LastUpdateTime.Format(time.RFC3339)
	}
	return ""
}

func (s StatefulSet) GetOriginal() interface{} {
	return s.StatefulSet
}

func (s StatefulSet) GetPodTemplate() corev1.PodTemplateSpec {
	return s.Spec.Template
}

func (s StatefulSet) getDeployedDate() string {
	readyIdx := slices.IndexFunc(s.Status.Conditions, func(c appsv1.StatefulSetCondition) bool { return c.Type == "OK" })
	if readyIdx > -1 {
		return s.Status.Conditions[readyIdx].LastTransitionTime.Format(time.RFC3339)
	}
	return ""
}

func (d DaemonSet) GetOriginal() interface{} {
	return d.DaemonSet
}

func (d DaemonSet) GetPodTemplate() corev1.PodTemplateSpec {
	return d.Spec.Template
}

func (d DaemonSet) getDeployedDate() string {
	readyIdx := slices.IndexFunc(d.Status.Conditions, func(c appsv1.DaemonSetCondition) bool { return c.Type == "OK" })
	if readyIdx > -1 {
		return d.Status.Conditions[readyIdx].LastTransitionTime.Format(time.RFC3339)
	}
	return ""
}

func (j CronJob) GetOriginal() interface{} {
	return j.CronJob
}

func (j CronJob) GetPodTemplate() corev1.PodTemplateSpec {
	return j.Spec.JobTemplate.Spec.Template
}

func (j CronJob) getDeployedDate() string {
	if j.CronJob.Status.LastScheduleTime != nil {
		return j.CronJob.Status.LastScheduleTime.Format(time.RFC3339)
	} else {
		return ""
	}
}

func (rsc *MetaResource) GetJsonPath(path string) (string, error) {
	return renderJsonPath(rsc.Listenable.GetOriginal(), path)
}

func (rsc *MetaResource) GetLabel(label string) string {
	return rsc.GetLabels()[label]
}

func (rsc *MetaResource) GetNsLabel(label string) string {
	return rsc.ns.GetLabels()[label]
}

func (rsc *MetaResource) GetNsAnnotation(annotation string) string {
	return rsc.ns.GetAnnotations()[annotation]
}

func (rsc *MetaResource) GetNsJsonPath(path string) (string, error) {
	return renderJsonPath(rsc.ns, path)
}

func (rsc *MetaResource) GetAnnotation(annotation string) string {
	return rsc.GetAnnotations()[annotation]
}

func (rsc *MetaResource) GetMainImageTag() (string, error) {
	if len(rsc.GetPodTemplate().Spec.Containers) == 0 {
		return "", nil
	}
	if image, err := parseContainerImage(rsc.GetPodTemplate().Spec.Containers[0].Image); err != nil {
		return "", err
	} else {
		return image.tag, nil
	}
}

func (rsc *MetaResource) GetMainImageName() (string, error) {
	if len(rsc.GetPodTemplate().Spec.Containers) == 0 {
		return "", nil
	}
	if image, err := parseContainerImage(rsc.GetPodTemplate().Spec.Containers[0].Image); err != nil {
		return "", err
	} else {
		return image.image, nil
	}
}

func ToListenable(obj interface{}) (Listenable, error) {
	switch obj.(type) {
	case *appsv1.Deployment:
		return Deployment{obj.(*appsv1.Deployment)}, nil
	case *appsv1.StatefulSet:
		return StatefulSet{obj.(*appsv1.StatefulSet)}, nil
	case *appsv1.DaemonSet:
		return DaemonSet{obj.(*appsv1.DaemonSet)}, nil
	case *batchv1.CronJob:
		return CronJob{obj.(*batchv1.CronJob)}, nil
	default:
		return nil, fmt.Errorf("object does not implement the Deployment, StatefulSet or DaemonSet: %q", obj)
	}
}

func NewMetaResource(obj interface{}, ns *corev1.Namespace) (*MetaResource, error) {
	listenable, err := ToListenable(obj)
	if err != nil {
		return nil, err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	typeAccessor, err := meta.TypeAccessor(obj)
	if err != nil {
		return nil, err
	}
	return &MetaResource{
		ns,
		accessor,
		typeAccessor,
		listenable,
	}, nil
}

func MetaStatus(resource *MetaResource) (EnvironmentStatus, error) {
	obj := resource.Listenable.GetOriginal()
	switch obj.(type) {
	case *appsv1.Deployment:
		o, _ := obj.(*appsv1.Deployment)
		return DeploymentStatus(o), nil
	case *appsv1.DaemonSet:
		o, _ := obj.(*appsv1.DaemonSet)
		return DamonSetStatus(o), nil
	case *appsv1.StatefulSet:
		o, _ := obj.(*appsv1.StatefulSet)
		return StatefulSetStatus(o), nil
	case *batchv1.CronJob:
		o, _ := obj.(*batchv1.CronJob)
		return CronJobStatus(o), nil
	default:
		return Unknown, fmt.Errorf("resource does not implement the Deployment, StatefulSet or DaemonSet: %q", obj)
	}
}

func DeploymentStatus(deployment *appsv1.Deployment) EnvironmentStatus {
	if deployment.Status.Replicas == 0 {
		return Down
	}
	if deployment.Status.ObservedGeneration < deployment.Generation {
		// // rsc being updated // TODO deploy ?
		return Unknown
	}
	cond := getCondition(deployment, appsv1.DeploymentProgressing)
	if cond != nil && cond.Reason == TimedOutReason {
		return Failed
	}
	if deployment.Spec.Replicas != nil && deployment.Status.UpdatedReplicas < *deployment.Spec.Replicas {
		// not all replicas updated
		return Deploy
	}
	if deployment.Status.UpdatedReplicas < deployment.Status.Replicas {
		// not all old replicas terminated
		return Deploy
	}
	if deployment.Status.AvailableReplicas < deployment.Status.UpdatedReplicas {
		// not all replicas ready
		return Deploy
	}
	return Up
}

func DamonSetStatus(daemon *appsv1.DaemonSet) EnvironmentStatus {
	if daemon.Status.DesiredNumberScheduled == 0 {
		return Down
	}
	if daemon.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		// status only available for rolling update TODO not for env
		return Unknown
	}
	if daemon.Status.ObservedGeneration < daemon.Generation {
		// rsc being updated // TODO deploy ?
		return Unknown
	}
	if daemon.Status.UpdatedNumberScheduled < daemon.Status.DesiredNumberScheduled {
		// not all daemon updated
		return Deploy
	}
	if daemon.Status.NumberReady < daemon.Status.DesiredNumberScheduled {
		// not all daemon available
		return Deploy
	}
	return Up
}

func StatefulSetStatus(sts *appsv1.StatefulSet) EnvironmentStatus {
	if sts.Status.Replicas == 0 {
		return Down
	}
	if sts.Spec.UpdateStrategy.Type != appsv1.RollingUpdateStatefulSetStrategyType {
		// status only available for rolling update TODO not for env
		return Unknown
	}
	if sts.Status.ObservedGeneration == 0 || sts.Status.ObservedGeneration <= sts.Generation {
		// rsc being updated // TODO deploy ?
		return Unknown
	}
	if sts.Spec.Replicas != nil && sts.Status.ReadyReplicas < *sts.Spec.Replicas {
		// not all pod ready
		return Deploy
	}
	if sts.Spec.Replicas != nil && sts.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		if sts.Status.UpdatedReplicas < (*sts.Spec.Replicas - *sts.Spec.UpdateStrategy.RollingUpdate.Partition) {
			// not all pod updated
			return Deploy
		}
	}
	if sts.Status.UpdateRevision != sts.Status.CurrentRevision {
		// not all pod in last revision
		return Deploy
	}
	return Up
}

func CronJobStatus(cron *batchv1.CronJob) EnvironmentStatus {
	// TODO how to ?
	return Unknown
}

func revision(deployment *appsv1.Deployment) (int64, error) {
	acc, err := meta.Accessor(deployment)
	if err != nil {
		return 0, err
	}
	r := acc.GetAnnotations()[RevisionAnnotation]
	return strconv.ParseInt(r, 10, 64)
}

func getCondition(deployment *appsv1.Deployment, condtype appsv1.DeploymentConditionType) *appsv1.DeploymentCondition {
	for _, cond := range deployment.Status.Conditions {
		if cond.Type == condtype {
			return &cond
		}
	}
	return nil
}
