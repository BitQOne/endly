package endly_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/viant/endly"
	"github.com/viant/toolbox"
	"log"
	"os/exec"
	"path"
	"testing"
)

func TestCliRunner_RunDsUnitWorkflow(t *testing.T) {

	exec.Command("rm", "-rf", "/tmp/endly/test/workflow/dsunit").CombinedOutput()
	toolbox.CreateDirIfNotExist("/tmp/endly/test/workflow/dsunit")
	runner := endly.NewCliRunner()
	err := runner.Run("test/runner/run_dsunit.json")
	if err != nil {
		log.Fatal(err)
	}
}

func TestCliRunner_RunDsHttpWorkflow(t *testing.T) {

	baseDir := toolbox.CallerDirectory(3)
	err := endly.StartHTTPServer(8120, &endly.HTTPServerTrips{
		IndexKeys:     []string{endly.MethodKey, endly.URLKey, endly.BodyKey, endly.CookieKey, endly.ContentTypeKey},
		BaseDirectory: path.Join(baseDir, "test/http/runner/http_workflow"),
	})

	if !assert.Nil(t, err) {
		return
	}
	exec.Command("rm", "-rf", "/tmp/endly/test/workflow/dsunit").CombinedOutput()
	toolbox.CreateDirIfNotExist("/tmp/endly/test/workflow/dsunit")
	runner := endly.NewCliRunner()
	err = runner.Run("test/runner/run_http.json")
	if err != nil {
		log.Fatal(err)
	}
}
