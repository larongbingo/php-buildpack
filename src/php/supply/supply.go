package supply

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cloudfoundry/libbuildpack"
)

type Stager interface {
	//TODO: See more options at https://github.com/cloudfoundry/libbuildpack/blob/master/stager.go
	BuildDir() string
	CacheDir() string
	DepDir() string
	DepsIdx() string
	DepsDir() string
	LinkDirectoryInDepDir(string, string) error
	WriteProfileD(string, string) error
}

type Manifest interface {
	//TODO: See more options at https://github.com/cloudfoundry/libbuildpack/blob/master/manifest.go
	AllDependencyVersions(string) []string
	DefaultVersion(string) (libbuildpack.Dependency, error)
	FetchDependency(libbuildpack.Dependency, string) error
	InstallDependency(libbuildpack.Dependency, string) error
	InstallOnlyVersion(string, string) error
	RootDir() string
}

type Command interface {
	//TODO: See more options at https://github.com/cloudfoundry/libbuildpack/blob/master/command.go
	Execute(string, io.Writer, io.Writer, string, ...string) error
	Output(dir string, program string, args ...string) (string, error)
	Run(cmd *exec.Cmd) error
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
	if err := s.WriteProfileD(); err != nil {
		s.Log.Error("Failed to write profile.d: %s", err)
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

func (s *Supplier) InstallComposer() error {
	depVersions := s.Manifest.AllDependencyVersions("composer")
	if len(depVersions) != 1 {
		return fmt.Errorf("expected 1 version of composer, found %d", len(depVersions))
	}
	s.Log.BeginStep("Installing composer %s", depVersions[0])
	dep := libbuildpack.Dependency{Name: "composer", Version: depVersions[0]}
	if err := s.Manifest.FetchDependency(dep, "/tmp/composer.phar"); err != nil {
		return err
	}

	// php composer-setup.php --install-dir=bin --filename=composer
	if output, err := s.Command.Output(s.Stager.DepDir(), "php", "/tmp/composer.phar", "--install-dir=composer", "--filename=composer"); err != nil {
		s.Log.Error(output)
		return err
	}
	return os.Remove("/tmp/composer.phar")
}

// [php_app] 2018-03-18T20:29:56.963471900Z 2018-03-18 20:29:56,959 [DEBUG] composer - Running command [/tmp/app/php/bin/php /tmp/app/php/bin/composer.phar install --no-progress --no-interaction --no-dev]
// [php_app] 2018-03-18T20:29:56.963523300Z 2018-03-18 20:29:56,959 [DEBUG] composer - ENV IS: COMPOSER_CACHE_DIR=/tmp/cache/final/composer (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963739200Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: COMPOSER_VENDOR_DIR=/tmp/app/lib/vendor (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963750900Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: PHPRC=/tmp (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963797300Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: COMPOSER_BIN_DIR=/tmp/app/php/bin (<type 'str'>)
func (s *Supplier) RunComposer() error {
	cmd := exec.Command("php", "composer", "install", "--no-progress", "--no-interaction", "--no-dev")
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("COMPOSER_CACHE_DIR=%s/composer", s.Stager.CacheDir()),
		// "PHPRC=/tmp",
		// fmt.Sprintf("COMPOSER_VENDOR_DIR=%s/lib/vendor", s.Stager.BuildDir()),
		fmt.Sprintf("COMPOSER_BIN_DIR=%s/php/bin", s.Stager.CacheDir()),
	)
	cmd.Dir = s.Stager.BuildDir()
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
