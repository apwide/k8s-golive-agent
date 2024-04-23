package k8s

import (
	"context"
	"github.com/spf13/viper"
	"k8s.io/klog/v2"
	"strings"
)

type AttributeDefinition struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	Secured bool   `yaml:"secured"`
}

type NameReferenceSource struct {
	Value      string `yaml:"value"`
	Label      string `yaml:"label"`
	Annotation string `yaml:"annotation"`
}

type NamespacedSource struct {
	Namespace bool `yaml:"namespace"`
}

type CombinedReferenceSource struct {
	NameReferenceSource `yaml:"NameReferenceSource,inline"`
	NamespacedSource    `yaml:"NamespacedSource,inline"`
}

type VersionSource struct {
	CombinedReferenceSource `yaml:"NameReferenceSource,inline"`
	Prefix                  string `yaml:"prefix"`
	Suffix                  string `yaml:"suffix"`
	Ignore                  bool   `yaml:"ignore"`
}

type AttributeSource struct {
	Name     string `yaml:"name"`
	Value    string `yaml:"value"`
	FromPath string `yaml:"fromPath"`
}

// Selector are applied on pod's
type Selector struct {
	Name       string            `yaml:"name"` // makes no sens on pod
	Namespace  string            `yaml:"namespace"`
	LabelQuery string            `yaml:"labelQuery"`
	Labels     map[string]string `yaml:"labels"`
}

type NamedReference struct {
	Id   *int32  `yaml:"id"`
	Name *string `yaml:"name"`
}

type StatusMapping struct {
	Down   NamedReference `yaml:"down"`
	Deploy NamedReference `yaml:"deploy"`
	Failed NamedReference `yaml:"failed"`
	Up     NamedReference `yaml:"up"`
}

type Listener struct {
	Id          string                  `yaml:"id"`
	AutoCreate  bool                    `yaml:"autoCreate"`
	Category    CombinedReferenceSource `yaml:"category"`
	Application CombinedReferenceSource `yaml:"application"`
	Name        CombinedReferenceSource `yaml:"name"`
	Version     VersionSource           `yaml:"version"`
	Selectors   []Selector              `yaml:"selectors" validate:"min=1"`
	Attributes  []AttributeSource       `yaml:"attributes"`
}

type Config struct {
	Golive struct {
		Url      string `yaml:"url"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Token    string `yaml:"token"`
	} `yaml:"golive"`
	Initialize struct {
		EnvironmentAttributes []AttributeDefinition `yaml:"environmentAttributes"`
		DeploymentAttributes  []AttributeDefinition `yaml:"deploymentAttributes"`
	} `yaml:"initialize"`
	StatusMapping *StatusMapping `yaml:"statusMapping"`
	Listeners     []Listener     `yaml:"listeners"`
}

func (l *Listener) FixedAttributes() map[string]string {
	attributes := make(map[string]string)
	for _, attribute := range l.Attributes {
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
