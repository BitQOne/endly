package endly_test

import (
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/viant/endly"
	"github.com/viant/toolbox"
	"testing"
)

func assertWithService(expected, actual interface{}) (int, error) {
	manager := endly.NewManager()
	service, err := manager.Service(endly.ValidatorServiceID)
	if err != nil {
		return 0, err
	}
	context := manager.NewContext(toolbox.NewContext())
	response := service.Run(context, &endly.ValidatorAssertRequest{
		Expected: expected,
		Actual:   actual,
	})

	if response.Error != "" {
		return 0, errors.New(response.Error)
	}

	validationResponse, ok := response.Response.(*endly.ValidationInfo)
	if !ok {
		return 0, nil
	}
	if len(validationResponse.FailedTests) > 0 {
		return 0, errors.New(validationResponse.Message())
	}
	return validationResponse.TestPassed, nil
}

func TestValidatorService_Assert(t *testing.T) {

	{
		passed, err := assertWithService("abc", "abc")
		assert.Nil(t, err)
		assert.Equal(t, 1, passed)
	}
	{
		passed, _ := assertWithService("abc", "abcd")
		assert.Equal(t, 0, passed)
		assert.Equal(t, 0, passed)
	}
	{
		passed, err := assertWithService("/abc/", "abcd")
		assert.Nil(t, err)
		assert.Equal(t, 1, passed)
	}

	{
		passed, err := assertWithService("/!abc/", "abcd")
		assert.NotNil(t, err)
		assert.Equal(t, 0, passed)
	}

	{
		passed, err := assertWithService("~/.+(\\d+).+/", "avc1erwer")
		assert.Nil(t, err)
		assert.Equal(t, 1, passed)
	}

	{
		passed, err := assertWithService("~/!.+(\\d+).+/", "avc1erwer")
		assert.NotNil(t, err)
		assert.Equal(t, 0, passed)
	}

	{
		passed, err := assertWithService("~/.+(\\d+).+/", "avc1erw\ner")
		assert.Nil(t, err)
		assert.Equal(t, 0, passed)
	}

	{
		passed, err := assertWithService("123.4343", 123.4343)
		assert.Nil(t, err)
		assert.Equal(t, 1, passed)
	}

	{
		passed, err := assertWithService([]string{
			"abc", "/a/",
		}, []interface{}{
			"abc", "abc",
		})
		assert.Nil(t, err)
		assert.Equal(t, 2, passed)
	}

	{
		passed, err := assertWithService(map[string]string{
			"k1": "abc",
			"k2": "/a/",
		}, map[string]interface{}{
			"k1": "abc",
			"k2": "abc",
			"k3": "wewewq", //k3 was not expected but no listed  -> use does not exist directive
		})
		assert.Nil(t, err)
		assert.Equal(t, 2, passed)
	}
	{
		passed, err := assertWithService(map[string]string{
			"k1": "abc",
			"k2": "/a/",
			"k3": "@exists@",
		}, map[string]interface{}{
			"k1": "abc",
			"k2": "abc",
			"k3": "wewewq", //k3 was not expected but no listed  -> use does not exist directive
		})
		assert.Nil(t, err)
		assert.Equal(t, 3, passed)
	}

	{
		passed, err := assertWithService(map[string]string{
			"k1": "abc",
			"k2": "/a/",
			"k3": "@!exists@",
		}, map[string]interface{}{
			"k1": "abc",
			"k2": "abc",
			"k3": "wewewq", //k3 was not expected but no listed  -> use does not exist directive
		})
		assert.NotNil(t, err)
		assert.Equal(t, 0, passed)
	}

	{
		passed, err := assertWithService(map[string]interface{}{
			"@indexBy@": "name",
			"k1": map[string]string{
				"name":  "k1",
				"value": "v1",
			},
			"k2": map[string]string{
				"name":  "k2",
				"value": "v2",
			},
		}, []interface{}{
			map[string]string{
				"name":  "k1",
				"value": "v1",
			},
			map[string]string{
				"name":  "k2",
				"value": "v2",
			},
		})
		assert.Nil(t, err)
		assert.Equal(t, 5, passed)
	}

}
