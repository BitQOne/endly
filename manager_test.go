package endly_test

import (
	"github.com/stretchr/testify/assert"
	"github.com/viant/endly"
	"github.com/viant/toolbox"
	"github.com/viant/toolbox/url"
	"reflect"
	"testing"
)

func TestNewManager(t *testing.T) {

	manager := endly.NewManager()
	context := manager.NewContext(toolbox.NewContext())
	manager.Register(newTestService())

	service, err := manager.Service("testService")
	assert.Nil(t, err)
	assert.NotNil(t, service)

	_, err = manager.Service("cc")
	assert.NotNil(t, err)

	manager2, err := context.Manager()
	assert.Nil(t, err)
	assert.Equal(t, manager2, manager)

	{
		service, err := manager2.Service("testService")
		assert.Nil(t, err)
		assert.NotNil(t, service)

	}

	{
		service, err := context.Service("testService")
		assert.Nil(t, err)
		assert.NotNil(t, service)

	}

	{
		state := context.State()
		assert.NotNil(t, state)
		state.Put("k1", 1)
	}
	{
		state := context.State()
		assert.Equal(t, 1, state.GetInt("k1"))
	}

}

type testService struct {
	*endly.AbstractService
}

func (t *testService) Run(context *endly.Context, request interface{}) *endly.ServiceResponse {
	return &endly.ServiceResponse{}
}

func newTestService() endly.Service {
	var result = &testService{
		AbstractService: endly.NewAbstractService("testService"),
	}
	result.AbstractService.Service = result
	return result

}

func Test_ServiceRequest(t *testing.T) {

	var invalidResourse = &url.Resource{URL: "a d:/sdwe/23/e"}

	manager := endly.NewManager()
	context := manager.NewContext(toolbox.NewContext())
	var services = endly.Services(manager)
	for k, service := range services {
		_, err := service.NewRequest("abc")
		assert.NotNil(t, err, k)
		response := service.Run(context, struct{}{})
		assert.True(t, response.Error != "", k)

		for _, action := range service.Actions() {
			request, err := service.NewRequest(action)
			assert.Nil(t, err)
			assert.NotNil(t, request)
			if _, ok := request.(Validator); ok {
				response = service.Run(context, request)
				assert.True(t, response.Error != "")
			}

			var requestType = toolbox.DereferenceType(reflect.TypeOf(request))

			if requestType.Kind() == reflect.Struct {
				if _, has := requestType.FieldByName("Target"); has {
					reflect.ValueOf(request).Elem().FieldByName("Target").Set(reflect.ValueOf(invalidResourse))

					response = service.Run(context, request)
					assert.True(t, response.Error != "")
				}
			}

		}

	}

}

type Validator interface {
	Vaidate() error
}
