package python

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jumpscale/go-raml/raml"
	. "github.com/smartystreets/goconvey/convey"
)

func TestGeneratePythonClientFromRaml(t *testing.T) {
	Convey("Python client", t, func() {
		apiDef := new(raml.APIDefinition)
		err := raml.ParseFile("./fixtures/client/client.raml", apiDef)
		So(err, ShouldBeNil)

		targetDir, err := ioutil.TempDir("", "")
		So(err, ShouldBeNil)

		Convey("requests client", func() {
			client := NewClient(apiDef, "")
			err = client.Generate(targetDir)
			So(err, ShouldBeNil)

			rootFixture := "./fixtures/client/requests_client"
			// cek with generated with fixtures
			checks := []struct {
				Result   string
				Expected string
			}{
				{"client.py", "client.py"},
				{"__init__.py", "__init__.py"},
				{"client_utils.py", "client_utils.py"},
				{"users_service.py", "users_service.py"},
			}

			for _, check := range checks {
				s, err := testLoadFile(filepath.Join(targetDir, check.Result))
				So(err, ShouldBeNil)

				tmpl, err := testLoadFile(filepath.Join(rootFixture, check.Expected))
				So(err, ShouldBeNil)

				So(s, ShouldEqual, tmpl)
			}
		})

		Convey("aiohttp client", func() {
			client := NewClient(apiDef, clientNameAiohttp)
			err = client.Generate(targetDir)
			So(err, ShouldBeNil)

			rootFixture := "./fixtures/client/aiohttp_client"
			// cek with generated with fixtures
			files := []string{
				"client.py",
				"__init__.py",
				"client_utils.py",
				"users_service.py",
			}

			for _, f := range files {
				s, err := testLoadFile(filepath.Join(targetDir, f))
				So(err, ShouldBeNil)

				tmpl, err := testLoadFile(filepath.Join(rootFixture, f))
				So(err, ShouldBeNil)

				So(s, ShouldEqual, tmpl)
			}
		})

		Reset(func() {
			os.RemoveAll(targetDir)
		})
	})
}

func testLoadFile(filename string) (string, error) {
	b, err := ioutil.ReadFile(filename)
	return string(b), err
}
