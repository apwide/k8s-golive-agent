package k8s

import (
	"bytes"
	"fmt"
	"k8s.io/client-go/util/jsonpath"
	"regexp"
)

func extractJsonPathValue(d *MetaResource, jsonPath string) (string, error) {
	template := jsonpath.New("template")
	err := template.Parse("{ " + jsonPath + " }")
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	err = template.Execute(buf, d.Listenable.GetOriginal())
	return buf.String(), err
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

func truncate(value string, max int) string {
	if max > len(value) {
		return value
	}
	return value[:max]
}
