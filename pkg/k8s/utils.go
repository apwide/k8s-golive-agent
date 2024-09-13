package k8s

import (
	"bytes"
	"fmt"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"k8s.io/client-go/util/jsonpath"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
)

func renderJsonPathFromMeta(d *MetaResource, jsonPath string) (string, error) {
	return renderJsonPath(d.Listenable.GetOriginal(), jsonPath)
}

func renderJsonPath(obj interface{}, jsonPath string) (string, error) {
	template := jsonpath.New("template")
	err := template.Parse("{" + jsonPath + "}")
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	err = template.Execute(buf, obj)
	if err != nil {
		return "", err
	}
	str := buf.String()
	// when jsonpath result is an object (eg: Quantity for resource limit), sometimes, it uses objet UnMarshal for results which adds quote
	if strings.HasPrefix(str, "\"") && strings.HasSuffix(str, "\"") {
		str = strings.TrimPrefix(strings.TrimSuffix(str, "\""), "\"")
	}
	return str, err
}

type dockerImage struct {
	repository string
	image      string
	tag        string
}

func parseContainerImage(image string) (*dockerImage, error) {
	expr := regexp.MustCompile("^(?P<repository>(?:[\\w-_]+\\.)+(?:[\\w-_]+)(?:\\:\\d+)?)?(/)?(?P<image>[\\w-_/]+)(:)?(?P<tag>.*)$")
	matches := expr.FindStringSubmatch(image)
	if len(matches) > 0 {
		return &dockerImage{
			repository: matches[expr.SubexpIndex("repository")],
			image:      matches[expr.SubexpIndex("image")],
			tag:        matches[expr.SubexpIndex("tag")],
		}, nil
	}
	return nil, fmt.Errorf("unable to parse container image %q", image)
}

func renderTemplate(input string, resource *MetaResource, defaultKey string, ctx map[string]interface{}) (string, error) {
	funcs := template.FuncMap{
		"mainImageName": resource.GetMainImageName,
		"mainImageTag":  resource.GetMainImageTag,
		"jsonPath":      resource.GetJsonPath,
		"label":         resource.GetLabel,
		"name":          resource.GetName,
		"defaultLabel": func() string {
			if defaultKey != None {
				return resource.GetLabel(defaultKey)
			} else {
				return None
			}
		},
		"defaultAnnotation": func() string {
			if defaultKey != None {
				return resource.GetAnnotation(defaultKey)
			} else {
				return None
			}
		},
		"nsLabel":      resource.GetNsLabel,
		"nsName":       resource.ns.GetName,
		"nsAnnotation": resource.GetNsAnnotation,
		"nsDefaultLabel": func() string {
			if defaultKey != None {
				return resource.GetNsLabel(defaultKey)
			} else {
				return None
			}
		},
		"nsDefaultAnnotation": func() string {
			if defaultKey != None {
				return resource.GetNsAnnotation(defaultKey)
			} else {
				return None
			}
		},
		"nsJsonPath": resource.GetNsJsonPath,
		"annotation": resource.GetAnnotation,
		"title":      cases.Title(language.English).String,
		"lower":      cases.Lower(language.English).String,
		"upper":      cases.Upper(language.English).String,
		"cutPrefix": func(value string, prefix string) string {
			out, _ := strings.CutPrefix(value, prefix)
			return out
		},
		"cutSuffix": func(value string, suffix string) string {
			out, _ := strings.CutSuffix(value, suffix)
			return out
		},
	}

	tpl, err := template.New("tpl").Funcs(funcs).Parse(strings.TrimSpace(input))
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	err = tpl.Execute(buf, ctx)
	return buf.String(), err
}

func truncate(value string, max int) string {
	if max > len(value) {
		return value
	}
	return value[:max]
}

func ExpandUserHome(path string) string {
	if path == "" {

	}
	usr, err := user.Current()
	if err != nil || usr == nil {
		return path
	}

	dir := usr.HomeDir

	if path == "~" {
		return dir
	} else if strings.HasPrefix(path, "~/") {
		return filepath.Join(dir, path[2:])
	} else {
		return path
	}
}
