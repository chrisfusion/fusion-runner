package runner

import "os"

type Config struct {
	MountPath    string
	Version      string
	Artifact     string
	Tag          string
	Namespace    string
	RunnerType   string
	Port         string
	Entrypoint   string
	BuilderImage string
	Maintainer   string
	IngressPath  string
	IndexURL     string
}

func LoadConfig() Config {
	return Config{
		MountPath:    getenv("WEAVE_MOUNT_PATH", "/weave-code"),
		Version:      getenv("WEAVE_VERSION", "unknown"),
		Artifact:     os.Getenv("WEAVE_ARTIFACT"),
		Tag:          os.Getenv("WEAVE_TAG"),
		Namespace:    os.Getenv("WEAVE_NAMESPACE"),
		RunnerType:   os.Getenv("WEAVE_RUNNER_TYPE"),
		Port:         os.Getenv("WEAVE_PORT"),
		Entrypoint:   os.Getenv("ENTRYPOINT"),
		BuilderImage: os.Getenv("WEAVE_BUILDER_IMAGE"),
		Maintainer:   os.Getenv("WEAVE_MAINTAINER"),
		IngressPath:  os.Getenv("WEAVE_INGRESS_PATH"),
		IndexURL:     os.Getenv("INDEX_URL"),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
