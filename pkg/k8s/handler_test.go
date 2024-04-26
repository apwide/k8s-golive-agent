package k8s

import (
	"context"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"testing"
)

func TestGetAttributes(t *testing.T) {
	annotations := make(map[string]string)
	annotations[AppKey] = "AppValue"
	annotations[CatKey] = "CatValue"
	annotations[EnvAttributePrefix+"CustomAttribute"] = "MyValue"

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: annotations,
		},
	}
	ns := corev1.Namespace{}
	resource, err := NewMetaResource(&deployment, &ns)
	assert.NoError(t, err, "Not able to convert deployment to meta resource")

	handler := Handler{
		logger:   klog.FromContext(context.Background()),
		listener: &Listener{},
	}
	attributes, err := handler.getEnvironmentAttributes(resource)

	assert.NoError(t, err, "Not able to read environment attributes")
	assert.Len(t, attributes, 1, "Should have only one attributes")
	assert.Equal(t, "MyValue", attributes["CustomAttribute"])

}
