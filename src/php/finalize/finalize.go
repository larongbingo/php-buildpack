package finalize

import (
	"fmt"
	"io"
	"io/ioutil"
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

	start := fmt.Sprintf(`#!/usr/bin/env bash
# TODO real process management
$DEPS_DIR/%s/httpd/bin/apachectl -f "$DEPS_DIR/%s/httpd/conf/httpd.conf" -k start -DFOREGROUND &
$DEPS_DIR/%s/php/sbin/php-fpm -p "$DEPS_DIR/%s/php/etc" -y "$DEPS_DIR/%s/php/etc/php-fpm.conf" -c "$DEPS_DIR/%s/php/etc"
`, f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx())
	if err := ioutil.WriteFile(filepath.Join(f.Stager.DepDir(), "start"), []byte(start), 0755); err != nil {
		f.Log.Error("Error writing start file: %v", err)
		return err
	}

	data, err := f.GenerateReleaseYaml()
	if err != nil {
		f.Log.Error("Error generating release YAML: %v", err)
		return err
	}
	return libbuildpack.NewYAML().Write("/tmp/php-buildpack-release-step.yml", data)
}

func (f *Finalizer) GenerateReleaseYaml() (map[string]map[string]string, error) {
	return map[string]map[string]string{
		"default_process_types": {
			"web": fmt.Sprintf("$DEPS_DIR/%s/start", f.Stager.DepsIdx()),
		},
	}, nil
}
