package supply

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	rice "github.com/GeertJohan/go.rice"
	"github.com/cloudfoundry/libbuildpack"
	"github.com/kr/text"
)

//go:generate rice embed-go

type Stager interface {
	BuildDir() string
	CacheDir() string
	DepDir() string
	DepsDir() string
	DepsIdx() string
	LinkDirectoryInDepDir(string, string) error
	WriteProfileD(string, string) error
}

type Manifest interface {
	AllDependencyVersions(string) []string
	DefaultVersion(string) (libbuildpack.Dependency, error)
	FetchDependency(libbuildpack.Dependency, string) error
	InstallDependency(libbuildpack.Dependency, string) error
	InstallOnlyVersion(string, string) error
	RootDir() string
}

type Command interface {
	Output(dir string, program string, args ...string) (string, error)
	Run(cmd *exec.Cmd) error
}
type YAML interface {
	Load(file string, obj interface{}) error
}

type Supplier struct {
	Manifest   Manifest
	Stager     Stager
	Command    Command
	Log        *libbuildpack.Logger
	YAML       YAML
	PhpVersion string
}

func (s *Supplier) Run() error {
	s.Log.BeginStep("Supplying php")

	if err := s.Setup(); err != nil {
		return fmt.Errorf("Initialiizing: %s", err)
	}

	if err := s.InstallHTTPD(); err != nil {
		return fmt.Errorf("Installing HTTPD: %s", err)
	}
	if err := s.InstallPHP(); err != nil {
		return fmt.Errorf("Installing PHP: %s", err)
	}
	if err := s.WriteConfigFiles(); err != nil {
		s.Log.Error("Error writing config files: %s", err)
		return err
	}

	if found, err := libbuildpack.FileExists(filepath.Join(s.Stager.BuildDir(), "composer.json")); err != nil {
		s.Log.Error("Error checking existence of composer.json: %s", err)
		return err
	} else if found {
		if err := s.InstallComposer(); err != nil {
			s.Log.Error("Failed to install composer: %s", err)
			return err
		}
		if err := s.RunComposer(); err != nil {
			s.Log.Error("Failed to run composer: %s", err)
			return err
		}
	}
	if err := s.InstallVarify(); err != nil {
		s.Log.Error("Failed to copy verify: %s", err)
		return err
	}
	if err := s.WriteProfileD(); err != nil {
		s.Log.Error("Failed to write profile.d: %s", err)
		return err
	}

	return nil
}

func (s *Supplier) Setup() error {
	// .bp-config/options.json
	var options struct {
		Version string `json:"PHP_VERSION"`
	}
	if err := s.YAML.Load(filepath.Join(s.Stager.BuildDir(), ".bp-config", "options.json"), &options); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else if options.Version != "" {
		m := regexp.MustCompile(`PHP_(\d)(\d)_LATEST`).FindStringSubmatch(options.Version)
		if len(m) == 3 {
			s.PhpVersion = fmt.Sprintf("%s.%s.x", m[1], m[2])
		} else {
			s.PhpVersion = options.Version
		}
	}

	// Composer.json
	var composer struct {
		Requires struct {
			Php string `json:"php"`
		} `json:"requires"`
	}
	if err := s.YAML.Load(filepath.Join(s.Stager.BuildDir(), "composer.json"), &composer); err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else if composer.Requires.Php != "" {
		if s.PhpVersion != "" {
			s.Log.Warning("A version of PHP has been specified in both `composer.json` and `./bp-config/options.json`.\nThe version defined in `composer.json` will be used.")
		}
		s.PhpVersion = composer.Requires.Php
	}

	if s.PhpVersion != "" {
		// Check version range
		versions := s.Manifest.AllDependencyVersions("php")
		if v, err := libbuildpack.FindMatchingVersion(s.PhpVersion, versions); err != nil {
			return err
		} else {
			s.PhpVersion = v
		}
	} else {
		// Default
		if dep, err := s.Manifest.DefaultVersion("php"); err != nil {
			return err
		} else {
			s.PhpVersion = dep.Version
		}
	}

	return nil
}

func (s *Supplier) InstallHTTPD() error {
	if err := s.Manifest.InstallOnlyVersion("httpd", s.Stager.DepDir()); err != nil {
		return err
	}
	for _, dir := range []string{"bin", "lib"} {
		if err := s.Stager.LinkDirectoryInDepDir(filepath.Join(s.Stager.DepDir(), "httpd", dir), dir); err != nil {
			return err
		}
	}
	return nil
}

func (s *Supplier) InstallPHP() error {
	dep := libbuildpack.Dependency{Name: "php", Version: s.PhpVersion}
	if err := s.Manifest.InstallDependency(dep, s.Stager.DepDir()); err != nil {
		return err
	}
	for _, dir := range []string{"bin", "lib"} {
		if err := s.Stager.LinkDirectoryInDepDir(filepath.Join(s.Stager.DepDir(), "php", dir), dir); err != nil {
			return err
		}
	}
	return nil
}

func (s *Supplier) WriteConfigFiles() error {
	ctxRun := map[string]string{
		"DepsIdx":           s.Stager.DepsIdx(),
		"PhpFpmConfInclude": "",
		"PhpFpmListen":      "127.0.0.1:9000",
		"Webdir":            "",
		"HOME":              "{{.HOME}}",
		"DEPS_DIR":          "{{.DEPS_DIR}}",
		"TMPDIR":            "{{.TMPDIR}}",
		// TODO should have stuff
		"PhpExtensions":  "extension=bz2.so\nextension=zlib.so\nextension=curl.so\nextension=mcrypt.so\nextension=openssl.so\n",
		"ZendExtensions": "",
	}
	ctxStage := make(map[string]string)
	for k, v := range ctxRun {
		ctxStage[k] = v
	}
	ctxStage["DEPS_DIR"] = s.Stager.DepsDir()
	ctxStage["HOME"] = s.Stager.BuildDir()
	ctxStage["TMPDIR"] = "/tmp"

	box := rice.MustFindBox("config")
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
			templateString = strings.Replace(templateString, "@{TMPDIR}", "{{.TMPDIR}}", -1)
			templateString = strings.Replace(templateString, "@{HOME}", "{{.HOME}}", -1)
			templateString = strings.Replace(templateString, "#PHP_FPM_LISTEN", "{{.PhpFpmListen}}", -1)
			tmplMessage, err := template.New(filepath.Join(src, destFile)).Parse(templateString)
			if err != nil {
				return err
			}

			for basedir, ctx := range map[string]map[string]string{s.Stager.DepDir(): ctxRun, "/tmp/php_etc": ctxStage} {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(basedir, dest, destFile)), 0755); err != nil {
					return err
				}
				fh, err := os.Create(filepath.Join(basedir, dest, destFile))
				if err != nil {
					return err
				}
				defer fh.Close()
				if err := tmplMessage.Execute(fh, ctx); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Supplier) InstallComposer() error {
	depVersions := s.Manifest.AllDependencyVersions("composer")
	if len(depVersions) != 1 {
		return fmt.Errorf("expected 1 version of composer, found %d", len(depVersions))
	}
	s.Log.BeginStep("Installing composer %s", depVersions[0])
	dep := libbuildpack.Dependency{Name: "composer", Version: depVersions[0]}
	return s.Manifest.FetchDependency(dep, filepath.Join(s.Stager.DepDir(), "bin", "composer"))
}

// [php_app] 2018-03-18T20:29:56.963471900Z 2018-03-18 20:29:56,959 [DEBUG] composer - Running command [/tmp/app/php/bin/php /tmp/app/php/bin/composer.phar install --no-progress --no-interaction --no-dev]
// [php_app] 2018-03-18T20:29:56.963523300Z 2018-03-18 20:29:56,959 [DEBUG] composer - ENV IS: COMPOSER_CACHE_DIR=/tmp/cache/final/composer (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963739200Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: COMPOSER_VENDOR_DIR=/tmp/app/lib/vendor (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963750900Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: PHPRC=/tmp (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963797300Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: COMPOSER_BIN_DIR=/tmp/app/php/bin (<type 'str'>)
func (s *Supplier) RunComposer() error {
	s.Log.BeginStep("Running composer")

	cmd := exec.Command("php", filepath.Join(s.Stager.DepDir(), "bin", "composer"), "install", "--no-progress", "--no-interaction", "--no-dev")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("COMPOSER_CACHE_DIR=%s/composer", s.Stager.CacheDir()),
		"PHPRC=/tmp/php_etc/php/etc",
		// fmt.Sprintf("PHPRC=%s/php/etc", s.Stager.DepDir()),
		fmt.Sprintf("COMPOSER_VENDOR_DIR=%s/lib/vendor", s.Stager.BuildDir()),
		fmt.Sprintf("COMPOSER_BIN_DIR=%s/php/bin", s.Stager.DepDir()),
		"TMPDIR=/tmp",
	)
	cmd.Dir = s.Stager.BuildDir()
	cmd.Stdout = text.NewIndentWriter(os.Stdout, []byte("       "))
	cmd.Stderr = text.NewIndentWriter(os.Stderr, []byte("       "))
	return s.Command.Run(cmd)
}

func (s *Supplier) InstallVarify() error {
	if exists, err := libbuildpack.FileExists(filepath.Join(s.Stager.DepDir(), "bin", "varify")); err != nil {
		return err
	} else if exists {
		return nil
	}

	return libbuildpack.CopyFile(filepath.Join(s.Manifest.RootDir(), "bin", "varify"), filepath.Join(s.Stager.DepDir(), "bin", "varify"))
}

func (s *Supplier) WriteProfileD() error {
	return s.Stager.WriteProfileD("bp_env_vars.sh", fmt.Sprintf("export PHPRC=$DEPS_DIR/%s/php/etc\nexport HTTPD_SERVER_ADMIN=admin@localhost\n", s.Stager.DepsIdx()))
}