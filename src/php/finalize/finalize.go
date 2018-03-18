package finalize

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

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

type Finalizer struct {
	Manifest Manifest
	Stager   Stager
	Log      *libbuildpack.Logger
}

func (f *Finalizer) Run() error {
	f.Log.BeginStep("Finalizing php")

	if err := f.SymlinkHttpd(); err != nil {
		f.Log.Error("Error symlinking httpd: %v", err)
		return err
	}

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

func (f *Finalizer) SymlinkHttpd() error {
	f.Log.BeginStep("Symlinking httpd into app dir")
	return os.Symlink(filepath.Join("..", "deps", f.Stager.DepsIdx(), "httpd"), filepath.Join(f.Stager.BuildDir(), "httpd"))
}

func (f *Finalizer) WriteConfigFiles() error {
	box := rice.MustFindBox("../../../defaults/config")
	for src, dest := range map[string]string{"php/5.6.x": "php/etc/", "httpd": "httpd/conf"} {
		err := box.Walk(src, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}
			destFile, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			templateString, err := box.String(filepath.Join(src, destFile))
			if err != nil {
				return err
			}
			templateString = strings.Replace(templateString, "@{DEPS_DIR}", "{{.DEPS_DIR}}", -1)
			templateString = strings.Replace(templateString, "@{HOME}", "{{.HOME}}", -1)
			templateString = strings.Replace(templateString, "#PHP_FPM_LISTEN", "{{.PhpFpmListen}}", -1)
			tmplMessage, err := template.New(filepath.Join(src, destFile)).Parse(templateString)
			if err != nil {
				return err
			}

			if err := os.MkdirAll(filepath.Dir(filepath.Join(f.Stager.DepDir(), dest, destFile)), 0755); err != nil {
				return err
			}
			fh, err := os.Create(filepath.Join(f.Stager.DepDir(), dest, destFile))
			if err != nil {
				return err
			}
			defer fh.Close()
			return tmplMessage.Execute(fh, map[string]string{
				"DepsIdx":           f.Stager.DepsIdx(),
				"PhpFpmConfInclude": "",
				"PhpFpmListen":      "127.0.0.1:9000",
				"Webdir":            "",
				"HOME":              "{{.HOME}}",
				"DEPS_DIR":          "{{.DEPS_DIR}}",
			})
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *Finalizer) WriteStartFile() error {
	start := fmt.Sprintf(`#!/usr/bin/env bash
varify "$DEPS_DIR/%s/php/etc/" "$DEPS_DIR/%s/httpd/conf/"
# TODO real process management
$DEPS_DIR/%s/php/sbin/php-fpm -p "$DEPS_DIR/%s/php/etc" -y "$DEPS_DIR/%s/php/etc/php-fpm.conf" -c "$DEPS_DIR/%s/php/etc" &
$DEPS_DIR/%s/httpd/bin/apachectl -f "$DEPS_DIR/%s/httpd/conf/httpd.conf" -k start -DFOREGROUND
`, f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx(), f.Stager.DepsIdx())
	return ioutil.WriteFile(filepath.Join(f.Stager.DepDir(), "start"), []byte(start), 0755)
}

func (f *Finalizer) GenerateReleaseYaml() (map[string]map[string]string, error) {
	return map[string]map[string]string{
		"default_process_types": {
			"web": fmt.Sprintf("$DEPS_DIR/%s/start", f.Stager.DepsIdx()),
		},
	}, nil
}
