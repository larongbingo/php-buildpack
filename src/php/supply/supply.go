package supply

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
	LinkDirectoryInDepDir(string, string) error
}

type Manifest interface {
	//TODO: See more options at https://github.com/cloudfoundry/libbuildpack/blob/master/manifest.go
	AllDependencyVersions(string) []string
	DefaultVersion(string) (libbuildpack.Dependency, error)
	InstallDependency(libbuildpack.Dependency, string) error
	InstallOnlyVersion(string, string) error
	RootDir() string
}

type Command interface {
	//TODO: See more options at https://github.com/cloudfoundry/libbuildpack/blob/master/command.go
	Execute(string, io.Writer, io.Writer, string, ...string) error
	Output(dir string, program string, args ...string) (string, error)
}

type Supplier struct {
	Manifest Manifest
	Stager   Stager
	Command  libbuildpack.Command
	Log      *libbuildpack.Logger
}

func (s *Supplier) Run() error {
	s.Log.BeginStep("Supplying php")

	if err := s.InstallHTTPD(); err != nil {
		return fmt.Errorf("Installing HTTPD: %s", err)
	}
	if err := s.InstallPHP(); err != nil {
		return fmt.Errorf("Installing PHP: %s", err)
	}
	if err := s.InstallVarify(); err != nil {
		s.Log.Error("Failed to copy verify: %s", err)
		return err
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
	dep, err := s.Manifest.DefaultVersion("php")
	if err != nil {
		return err
	}
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

func (s *Supplier) InstallVarify() error {
	if exists, err := libbuildpack.FileExists(filepath.Join(s.Stager.DepDir(), "bin", "varify")); err != nil {
		return err
	} else if exists {
		return nil
	}

	return libbuildpack.CopyFile(filepath.Join(s.Manifest.RootDir(), "bin", "varify"), filepath.Join(s.Stager.DepDir(), "bin", "varify"))
}
