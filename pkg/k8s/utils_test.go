package k8s

import (
	"github.com/stretchr/testify/assert"
	apps "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"os"
	"testing"
)

func loadRsc(path string) interface{} {
	file, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	decoder := serializer.NewCodecFactory(scheme.Scheme).UniversalDecoder()
	d := &apps.Deployment{}
	if err = runtime.DecodeInto(decoder, file, d); err != nil {
		panic(err)
	}
	return d
}

func loadMetaFrom(path string) *MetaResource {
	ns := &corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
	}
	if r, err := NewMetaResource(loadRsc(path), ns); err != nil {
		panic(err)
	} else {
		return r
	}
}

func TestSimpleJsonPath(t *testing.T) {
	r := loadMetaFrom("../../test/data/deployment.json")
	result, err := renderJsonPathFromMeta(r, ".metadata.namespace")
	assert.NoError(t, err, "should find namespace in json")
	assert.Equal(t, "golive-dev", result)
}

func TestComplexJsonPath(t *testing.T) {
	r := loadMetaFrom("../../test/data/deployment.json")
	result, err := renderJsonPathFromMeta(r, `$.spec.template.spec.containers[0].env[?(@.name=="APWIDE_DEPLOYMENT-MODE")].value`)
	assert.NoError(t, err, "should find env APWIDE_DEPLOYMENT-MODE in json")
	assert.Equal(t, "true", result)
}

func TestTemplateFromContext(t *testing.T) {
	r := loadMetaFrom("../../test/data/deployment.json")
	appName := "eCommerce"
	catName := "Dev"
	app := NameReference{
		Name: &appName,
	}
	cat := NameReference{
		Name: &catName,
	}
	ctx := make(map[string]interface{})
	ctx["App"] = app
	ctx["Cat"] = cat

	output, err := renderTemplate("{{ .App.Name }} - {{ .Cat.Name }}", r, AppKey, ctx)
	assert.NoError(t, err, "should not fail to render template")
	assert.Equal(t, appName+" - "+catName, output)
}

func TestTemplateFromContextWithLower(t *testing.T) {
	r := loadMetaFrom("../../test/data/deployment.json")
	appName := "eCommerce"
	catName := "Dev"
	app := NameReference{
		Name: &appName,
	}
	cat := NameReference{
		Name: &catName,
	}
	ctx := make(map[string]interface{})
	ctx["App"] = app
	ctx["Cat"] = cat

	output, err := renderTemplate(`{{ .App.Name }} {{.Cat.Name | lower }}`, r, AppKey, ctx)
	assert.NoError(t, err, "should not fail to render template")
	assert.Equal(t, "eCommerce dev", output)
}

func TestTemplateFromJsonPath(t *testing.T) {
	r := loadMetaFrom("../../test/data/deployment.json")
	ctx := make(map[string]interface{})

	output, err := renderTemplate(`{{ jsonPath ".metadata.namespace" }}`, r, AppKey, ctx)
	assert.NoError(t, err, "should not fail to render template")
	assert.Equal(t, "golive-dev", output)
}

func TestTemplateFromLabel(t *testing.T) {
	r := loadMetaFrom("../../test/data/deployment.json")
	ctx := make(map[string]interface{})

	output, err := renderTemplate(`{{ label "apwide.net/app" }}`, r, AppKey, ctx)
	assert.NoError(t, err, "should not fail to render template")
	assert.Equal(t, "golive-api", output)
}

func TestTemplateFromAnnotation(t *testing.T) {
	r := loadMetaFrom("../../test/data/deployment.json")
	ctx := make(map[string]interface{})

	output, err := renderTemplate(`{{ annotation "deployment.kubernetes.io/revision" }}`, r, AppKey, ctx)
	assert.NoError(t, err, "should not fail to render template")
	assert.Equal(t, "26", output)
}
