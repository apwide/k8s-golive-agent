package k8s

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestEmptyConfig(t *testing.T) {
	cfg, err := LoadConfig("../../test/data/config-empty.yaml")
	assert.Nil(t, err, "error unexpected", err)
	assert.Equal(t, 1, len(cfg.Listeners), "wrong number of listeners")
	assert.False(t, cfg.Listeners[0].Deployment.Ignore, "for version ignore")
	assert.Len(t, cfg.Listeners[0].Environment.Attributes, 0, "attribute snot empty")
}

func TestValue(t *testing.T) {
	cfg, err := LoadConfig("../../test/data/config-empty.yaml")
	assert.Nil(t, err, "error unexpected", err)
	assert.Equal(t, 1, len(cfg.Listeners), "wrong number of listeners")
	assert.Equal(t, "golive-dev", cfg.Listeners[0].Id, "wrong listener name")
	assert.Equal(t, "Dev", cfg.Listeners[0].Category.Name, "wrong category name")
}

func TestFullConfig(t *testing.T) {
	cfg, err := LoadConfig("../../test/data/config-full.yaml")
	assert.NoError(t, err, "error unexpected", err)
	listener := cfg.Listeners[0]
	assert.Equal(t, "golive-dev", listener.Selectors[0].Namespace, "wrong namespace")
	assert.Equal(t, 2, len(listener.Selectors[0].Labels), "wrong number of labels")
	assert.Equal(t, "golive-api", listener.Selectors[0].Labels["golive.apwide.net/app"], "wrong label value")
	assert.Equal(t, "Dev", listener.Selectors[0].Labels["golive.apwide.net/cat"], "wrong label value")
	assert.Equal(t, `{{ nsLabel "my.company.com/cat" }}`, strings.TrimSpace(listener.Category.Name), "wrong category name")
	assert.Equal(t, `{{ annotation "my.company.com/app" }}`, strings.TrimSpace(listener.Application.Name), "wrong application name")
	assert.Equal(t, "Cluster", listener.Environment.Attributes[0].Name, "wrong attribute name")
	assert.Equal(t, "DEV", listener.Environment.Attributes[0].Value, "wrong attribute value")
	assert.Equal(t, `$.spec.template.spec.containers[0].env[?(@.name=="APWIDE_DEPLOYMENT-MODE")].value`, listener.Environment.Attributes[2].Value, "wrong attribute fromPath multiline")
}
