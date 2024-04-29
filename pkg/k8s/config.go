package k8s

import (
	"context"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	"strings"
)

type AttributeDefinition struct {
	Name    string
	Type    string
	Secured bool
}

type NameReferenceSource struct {
	Name string
}

type DeploymentSource struct {
	VersionName string
	Ignore      bool
	Attributes  []AttributeSource
}

type AttributeSource struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// Selector are applied on pod's
type Selector struct {
	Name       string // makes no sens on pod
	Namespace  string
	LabelQuery string
	Labels     map[string]string
}

type NamedReference struct {
	Id   *int32
	Name *string
}

type StatusMapping struct {
	Down   NamedReference
	Deploy NamedReference
	Failed NamedReference
	Up     NamedReference
}

type EnvironmentSource struct {
	NameReferenceSource `mapstructure:",squash"`
	Url                 string
	Attributes          []AttributeSource
}

type Listener struct {
	Id          string
	AutoCreate  bool
	Category    NameReferenceSource
	Application NameReferenceSource
	Environment EnvironmentSource
	Deployment  DeploymentSource
	Selectors   []Selector
}

type Config struct {
	Golive struct {
		Url             string
		Username        string
		Password        string
		Token           string
		Offline         bool
		Yaml            bool
		CacheExpiration string
		DefaultReSync   string
	}
	Initialize struct {
		EnvironmentAttributes []AttributeDefinition
		DeploymentAttributes  []AttributeDefinition
	}
	StatusMapping *StatusMapping
	Listeners     []Listener
}

func (l *Listener) FixedAttributes() map[string]string {
	attributes := make(map[string]string)
	for _, attribute := range l.Environment.Attributes {
		if attribute.Value != "" {
			attributes[attribute.Name] = attribute.Value
		}
	}
	return attributes
}

func LoadConfig(path string) (cfg Config, err error) {
	logger := klog.FromContext(context.Background())
	logger.Info("Load Golive config", "path", path)
	//viper.AddConfigPath(path)
	//viper.SetConfigName("app")
	//viper.SetConfigType("env")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigFile(path)
	viper.AutomaticEnv()

	err = viper.ReadInConfig()
	if err != nil {
		return
	}
	err = viper.Unmarshal(&cfg)
	return
}
