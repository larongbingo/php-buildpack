package finalize

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/cloudfoundry/libbuildpack"
)

type Stager interface {
	//TODO: See more options at https://github.com/cloudfoundry/libbuildpack/blob/master/stager.go
	BuildDir() string
	DepDir() string
	DepsIdx() string
	DepsDir() string
}

type Manifest interface {
	//TODO: See more options at https://github.com/cloudfoundry/libbuildpack/blob/master/manifest.go
	AllDependencyVersions(string) []string
	DefaultVersion(string) (libbuildpack.Dependency, error)
	InstallDependency(libbuildpack.Dependency, string) error
	InstallOnlyVersion(string, string) error
}

type Command interface {
	//TODO: See more options at https://github.com/cloudfoundry/libbuildpack/blob/master/command.go
	Execute(string, io.Writer, io.Writer, string, ...string) error
	Output(dir string, program string, args ...string) (string, error)
}

type Finalizer struct {
	Manifest Manifest
	Stager   Stager
	Command  libbuildpack.Command
	Log      *libbuildpack.Logger
}

func (f *Finalizer) Run() error {
	f.Log.BeginStep("Configuring php")

	data, err := f.GenerateReleaseYaml()
	if err != nil {
		f.Log.Error("Error generating release YAML: %v", err)
		return err
	}
	releasePath := filepath.Join(f.Stager.DepDir(), "release-step.yml")
	libbuildpack.NewYAML().Write(releasePath, data)

	return nil
}

func (f *Finalizer) GenerateReleaseYaml() (map[string]map[string]string, error) {
	return map[string]map[string]string{
		"default_process_types": {
			"web": fmt.Sprintf("$DEPS_DIR/%d/start", f.Stager.DepsIdx()),
		},
	}, nil
}
