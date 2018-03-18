package finalize

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	rice "github.com/GeertJohan/go.rice"
	"github.com/cloudfoundry/libbuildpack"
)

//go:generate rice embed-go

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

	if err := f.WriteConfigFiles(); err != nil {
		f.Log.Error("Error writing config files: %v", err)
		return err
	}

	if err := f.WriteStartFile(); err != nil {
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

func (f *Finalizer) WriteConfigFiles() error {
	box := rice.MustFindBox("../../../defaults/config")
	templateString, err := box.String("php/5.6.x/php-fpm.conf")
	if err != nil {
		return err
	}
	templateString = strings.Replace(templateString, "@{DEPS_DIR}", "{{.DEPS_DIR}}", -1)
	templateString = strings.Replace(templateString, "@{HOME}", "{{.HOME}}", -1)
	tmplMessage, err := template.New("php/5.6.x/php-fpm.conf").Parse(templateString)
	if err != nil {
		return err
	}

	fh, err := os.Create(filepath.Join(f.Stager.DepDir(), "php", "etc", "php-fpm.conf"))
	if err != nil {
		return err
	}
	defer fh.Close()
	tmplMessage.Execute(fh, map[string]string{
		"DepsIdx":           f.Stager.DepsIdx(),
		"PhpFpmConfInclude": "",
		"Webdir":            "",
		"HOME":              "{{.HOME}}",
		"DEPS_DIR":          "{{.DEPS_DIR}}",
	})

	return nil
}

func (f *Finalizer) WriteStartFile() error {
	start := fmt.Sprintf(`#!/usr/bin/env bash
varify "$DEPS_DIR/%s/php/etc/php-fpm.conf"
# TODO real process management
$DEPS_DIR/%s/httpd/bin/apachectl -f "$DEPS_DIR/%s/httpd/conf/httpd.conf" -k start -DFOREGROUND &
$DEPS_DIR/%s/php/sbin/php-fpm -p "$DEPS_DIR/%s/php/etc" -y "$DEPS_DIR/%s/php/etc/php-fpm.conf" -c "$DEPS_DIR/%s/php/etc"
`, f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx())
	return ioutil.WriteFile(filepath.Join(f.Stager.DepDir(), "start"), []byte(start), 0755)
}

func (f *Finalizer) GenerateReleaseYaml() (map[string]map[string]string, error) {
	return map[string]map[string]string{
		"default_process_types": {
			"web": fmt.Sprintf("$DEPS_DIR/%s/start", f.Stager.DepsIdx()),
		},
	}, nil
}
