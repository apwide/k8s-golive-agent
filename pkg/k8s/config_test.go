package k8s

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestEmptyConfig(t *testing.T) {
	os.Setenv("GOLIVE_CONFIG", "../../test/data/config-empty.yaml")
	cfg, err := LoadConfig()
	assert.Nil(t, err, "error unexpected", err)
	assert.Equal(t, 1, len(cfg.Listeners), "wrong number of listeners")
	assert.False(t, cfg.Listeners[0].Version.Ignore, "for version ignore")
}

func TestValue(t *testing.T) {
	os.Setenv("GOLIVE_CONFIG", "../../test/data/config-empty.yaml")
	cfg, err := LoadConfig()
	assert.Nil(t, err, "error unexpected", err)
	assert.Equal(t, 1, len(cfg.Listeners), "wrong number of listeners")
	assert.Equal(t, "golive-dev", cfg.Listeners[0].Id, "wrong listener name")
	assert.Equal(t, "Dev", cfg.Listeners[0].Category.Value, "wrong category name")
}

func TestFullConfig(t *testing.T) {
	os.Setenv("GOLIVE_CONFIG", "../../test/data/config-full.yaml")
	cfg, err := LoadConfig()
	assert.Nil(t, err, "error unexpected", err)
	listener := cfg.Listeners[0]
	assert.Equal(t, "golive-dev", listener.Selectors[0].Namespace, "wrong namespace")
	assert.Equal(t, 2, len(listener.Selectors[0].Labels), "wrong number of labels")
	assert.Equal(t, "golive-api", listener.Selectors[0].Labels["golive.apwide.net/app"], "wrong label value")
	assert.Equal(t, "Dev", listener.Selectors[0].Labels["golive.apwide.net/cat"], "wrong label value")
	assert.True(t, listener.Category.Namespace, "wrong category namespace")
	assert.Equal(t, "my.company.com/cat", listener.Category.Label, "wrong category label")
	assert.Equal(t, "my.company.com/app", listener.Application.Annotation, "wrong application annotation")
	assert.Equal(t, "Cluster", listener.Attributes[0].Name, "wrong attribute name")
	assert.Equal(t, "DEV", listener.Attributes[0].Value, "wrong attribute value")
	assert.Equal(t, `$.spec.template.spec.containers[0].env[?(@.name=="APWIDE_DEPLOYMENT-MODE")].value`, listener.Attributes[2].FromPath, "wrong attribute fromPath multiline")
}
