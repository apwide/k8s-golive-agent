package k8s

import "testing"

func TestDockerImageParsing(t *testing.T) {
	img, err := parseContainerImage("docker.apwide.com/atlassian-host:latest")
	if err != nil {
		t.Errorf("failed to parse container image %q", err)
	}
	if img.image != "atlassian-host" {
		t.Errorf("expect image to equals atlassian-host but was %q", img.image)
	}
	if img.tag != "latest" {
		t.Errorf("expect tag to equals latest but was %q", img.tag)
	}
}

func TestDockerImageParsingWithoutTag(t *testing.T) {
	img, err := parseContainerImage("docker.apwide.com/atlassian-host")
	if err != nil {
		t.Errorf("failed to parse container image %q", err)
	}
	if img.image != "atlassian-host" {
		t.Errorf("expect image to equals atlassian-host but was %q", img.image)
	}
	if img.tag != "" {
		t.Errorf("expect tag to equals latest but was %q", img.tag)
	}
}

func TestDockerImageParsingWithoutRepository(t *testing.T) {
	img, err := parseContainerImage("atlassian-host:latest")
	if err != nil {
		t.Errorf("failed to parse container image %q", err)
	}
	if img.image != "atlassian-host" {
		t.Errorf("expect image to equals atlassian-host but was %q", img.image)
	}
	if img.tag != "latest" {
		t.Errorf("expect tag to equals latest but was %q", img.tag)
	}
}

func TestDockerImageParsingWithPort(t *testing.T) {
	img, err := parseContainerImage("docker.apwide.com:5001/atlassian-host:latest")
	if err != nil {
		t.Errorf("failed to parse container image %q", err)
	}
	if img.image != "atlassian-host" {
		t.Errorf("expect image to equals atlassian-host but was %q", img.image)
	}
	if img.tag != "latest" {
		t.Errorf("expect tag to equals latest but was %q", img.tag)
	}
}
