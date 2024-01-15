package k8s

import (
	"github.com/stretchr/testify/assert"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"testing"
)

func TestSimpleJsonPath(t *testing.T) {
	file, err := os.ReadFile("../../test/data/deployment.json")
	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder()
	d := &apps.Deployment{}
	err = runtime.DecodeInto(decoder, file, d)
	//err = json.Unmarshal(file, d)
	if err != nil {
		panic(err)
	}
	r, err := NewMetaResource(d)
	if err != nil {
		panic(err)
	}
	result, err := extractJsonPathValue(r, ".metadata.namespace")
	assert.Nil(t, err, "should find namespace in json")
	assert.Equal(t, "golive-dev", result)
}

func TestComplexJsonPath(t *testing.T) {
	file, err := os.ReadFile("../../test/data/deployment.json")
	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder()
	d := &apps.Deployment{}
	err = runtime.DecodeInto(decoder, file, d)
	r, _ := NewMetaResource(d)
	//err = json.Unmarshal(file, d)
	if err != nil {
		panic(err)
	}
	result, err := extractJsonPathValue(r, `$.spec.template.spec.containers[0].env[?(@.name=="APWIDE_DEPLOYMENT-MODE")].value`)
	assert.Nil(t, err, "should find env APWIDE_DEPLOYMENT-MODE in json")
	assert.Equal(t, "true", result)
}
