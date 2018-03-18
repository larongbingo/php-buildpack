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
	WriteProfileD(string, string) error
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
	// php composer-setup.php --install-dir=bin --filename=composer
	return nil
}

// [php_app] 2018-03-18T20:29:56.963471900Z 2018-03-18 20:29:56,959 [DEBUG] composer - Running command [/tmp/app/php/bin/php /tmp/app/php/bin/composer.phar install --no-progress --no-interaction --no-dev]
// [php_app] 2018-03-18T20:29:56.963509000Z 2018-03-18 20:29:56,959 [DEBUG] composer - ENV IS: CF_INSTANCE_PORT= (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963523300Z 2018-03-18 20:29:56,959 [DEBUG] composer - ENV IS: COMPOSER_CACHE_DIR=/tmp/cache/final/composer (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963535900Z 2018-03-18 20:29:56,959 [DEBUG] composer - ENV IS: USER=vcap (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963547800Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: HOME=/home/vcap (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963559600Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: PATH=/usr/local/bin:/usr/bin:/bin:/tmp/app/php/bin (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963571400Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: CF_STACK=cflinuxfs2 (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963583200Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: LD_LIBRARY_PATH=/tmp/app/php/lib (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963594900Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: LANG=en_US.UTF-8 (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963606500Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: MEMORY_LIMIT=1024m (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963618200Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: VCAP_APPLICATION={"application_id": "01d31c12-d066-495e-aca2-8d3403165360", "name": "php_app", "limits": {"mem": 1024, "fds": 16384, "disk": 4096}, "space_id": "18300c1c-1aa4-4ae7-81e6-ae59c6cdbaf1", "application_uris": ["localhost"], "version": "18300c1c-1aa4-4ae7-81e6-ae59c6cdbaf1", "application_name": "php_app", "space_name": "php_app-space", "application_version": "2b860df9-a0a1-474c-b02f-5985f53ea0bb", "uris": ["localhost"]} (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963643600Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: SHLVL=1 (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963656300Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: CF_INSTANCE_IP=0.0.0.0 (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963667900Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: VCAP_SERVICES={} (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963679500Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: CF_INSTANCE_PORTS=[] (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963691200Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: PYTHONPATH=/tmp/buildpacks/273590d2367b04189c35737bae24d470/lib (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963702900Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: CF_INSTANCE_ADDR= (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963715600Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: BUILDPACK_PATH=/tmp/buildpacks/273590d2367b04189c35737bae24d470 (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963727500Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: _=/usr/bin/python (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963739200Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: COMPOSER_VENDOR_DIR=/tmp/app/lib/vendor (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963750900Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: PHPRC=/tmp (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963762500Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: BP_DEBUG=true (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963774100Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: HOSTNAME=php_app (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963785700Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: PWD=/home/vcap (<type 'str'>)
// [php_app] 2018-03-18T20:29:56.963797300Z 2018-03-18 20:29:56,960 [DEBUG] composer - ENV IS: COMPOSER_BIN_DIR=/tmp/app/php/bin (<type 'str'>)
func (s *Supplier) RunComposer() error {
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

func (s *Supplier) WriteProfileD() error {
	return s.Stager.WriteProfileD("bp_env_vars.sh", fmt.Sprintf("export PHPRC=$DEPS_DIR/%s/php/etc\nexport HTTPD_SERVER_ADMIN=admin@localhost\n", s.Stager.DepsIdx()))
}
