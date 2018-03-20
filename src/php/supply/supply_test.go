package supply_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"php/supply"
	"syscall"

	"github.com/cloudfoundry/libbuildpack"
	"github.com/cloudfoundry/libbuildpack/ansicleaner"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	yaml "gopkg.in/yaml.v2"
)

//go:generate mockgen -source=supply.go --destination=mocks_test.go --package=supply_test

var _ = Describe("Supply", func() {
	var (
		err          error
		buildDir     string
		depsDir      string
		depsIdx      string
		supplier     *supply.Supplier
		logger       *libbuildpack.Logger
		buffer       *bytes.Buffer
		mockCtrl     *gomock.Controller
		mockManifest *MockManifest
		mockCommand  *MockCommand
		mockYAML     *MockYAML
	)

	BeforeEach(func() {
		buildDir, err = ioutil.TempDir("", "php-buildpack.build.")
		Expect(err).To(BeNil())

		depsDir, err = ioutil.TempDir("", "php-buildpack.deps.")
		Expect(err).To(BeNil())

		depsIdx = "9"
		Expect(os.MkdirAll(filepath.Join(depsDir, depsIdx), 0755)).To(Succeed())

		buffer = new(bytes.Buffer)

		logger = libbuildpack.NewLogger(ansicleaner.New(buffer))

		mockCtrl = gomock.NewController(GinkgoT())
		mockManifest = NewMockManifest(mockCtrl)
		mockCommand = NewMockCommand(mockCtrl)
		mockYAML = NewMockYAML(mockCtrl)

		args := []string{buildDir, "", depsDir, depsIdx}
		stager := libbuildpack.NewStager(args, logger, &libbuildpack.Manifest{})

		supplier = &supply.Supplier{
			Manifest: mockManifest,
			Stager:   stager,
			Command:  mockCommand,
			YAML:     mockYAML,
			Log:      logger,
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()

		err = os.RemoveAll(buildDir)
		Expect(err).To(BeNil())

		err = os.RemoveAll(depsDir)
		Expect(err).To(BeNil())
	})

	Describe("Setup", func() {
		Context("no app settings files", func() {
			BeforeEach(func() {
				mockYAML.EXPECT().Load(gomock.Any(), gomock.Any()).Return(os.NewSyscallError("", syscall.ENOENT)).Times(2)
				mockManifest.EXPECT().DefaultVersion("php").Return(libbuildpack.Dependency{Name: "php", Version: "1.3.5"}, nil)
			})
			It("sets php version from default php version", func() {
				Expect(supplier.Setup()).To(Succeed())
				Expect(supplier.PhpVersion).To(Equal("1.3.5"))
			})
		})
		Context("app has settings files, but no requested versions in them", func() {
			BeforeEach(func() {
				mockYAML.EXPECT().Load(gomock.Any(), gomock.Any()).Return(nil).Times(2)
				mockManifest.EXPECT().DefaultVersion("php").Return(libbuildpack.Dependency{Name: "php", Version: "1.3.5"}, nil)
			})
			It("sets php version from default php version", func() {
				Expect(supplier.Setup()).To(Succeed())
				Expect(supplier.PhpVersion).To(Equal("1.3.5"))
			})
		})
		Context("options.json has requested version", func() {
			BeforeEach(func() {
				mockYAML.EXPECT().Load(filepath.Join(buildDir, ".bp-config", "options.json"), gomock.Any()).Do(func(string, obj interface{}) error {
					return fmt.Errorf("My Error")
					// return yaml.Unmarshal([]byte(`{"PHP_VERSION":"2.3.4"}`), obj)
				})
				mockYAML.EXPECT().Load(filepath.Join(buildDir, "composer.json"), gomock.Any()).Return(os.NewSyscallError("", syscall.ENOENT))
			})
			FIt("sets php version", func() {
				Expect(supplier.Setup()).To(Succeed())
				Expect(supplier.PhpVersion).To(Equal("2.3.4"))
			})
		})
		Context("composer.json has requested version", func() {
			BeforeEach(func() {
				mockYAML.EXPECT().Load(filepath.Join(buildDir, ".bp-config", "options.json"), gomock.Any()).Return(os.NewSyscallError("", syscall.ENOENT))
				mockYAML.EXPECT().Load(filepath.Join(buildDir, "composer.json"), gomock.Any()).Do(func(string, obj interface{}) error {
					return yaml.Unmarshal([]byte(`{"requires":{"php":"3.4.5"}}`), obj)

					// x := Client{PrimaryContact:Contact{}}
					// v := reflect.ValueOf(&x)
					// fmt.Println("v type:", v.Type(), ", kind:", v.Kind())
					// f := v.Elem().FieldByName("PrimaryContact")
					// fmt.Println("f type:", f.Type(), ", kind:", f.Kind())
					// p := f.Addr()
					// fmt.Println("p type:", p.Type(), ", kind:", p.Kind())
					// p.Elem().FieldByName("Id").SetInt(1)
					// fmt.Println("Contact Id:", x.PrimaryContact.Id)
				})
			})
			It("sets php version", func() {
				Expect(supplier.Setup()).To(Succeed())
				Expect(supplier.PhpVersion).To(Equal("3.4.5"))
			})
		})
		// WARNING: A version of PHP has been specified in both `composer.json` and `./bp-config/options.json`.
		// WARNING: The version defined in `composer.json` will be used.
	})
})
