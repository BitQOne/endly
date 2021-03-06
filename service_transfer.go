package endly

import (
	"fmt"
	"github.com/viant/toolbox"
	"github.com/viant/toolbox/storage"
	"github.com/viant/toolbox/url"
	"io"
	"io/ioutil"
	url2 "net/url"
	"path"
	"strings"
)

//TODO refactor compress with https://golangcode.com/create-zip-files-in-go/

//TransferServiceID represents transfer service id
const TransferServiceID = "transfer"

//TransferServiceCopyAction represents a copy action
const TransferServiceCopyAction = "copy"

//UseMemoryService flag in the context to ignore
const UseMemoryService = "useMemoryService"

//CompressionTimeout compression/decompression timeout
var CompressionTimeout = 120000

//CopyEventType represents CopyEventType
type CopyEventType struct {
	SourceURL string
	TargetURL string
	Expand    bool
}

type transferService struct {
	*AbstractService
}

//NewExpandedContentHandler return a new reader that can substitute content with state map, replacement data provided in replacement map.
func NewExpandedContentHandler(context *Context, replaceMap map[string]string, expand bool) func(reader io.ReadCloser) (io.ReadCloser, error) {
	return func(reader io.ReadCloser) (io.ReadCloser, error) {
		defer reader.Close()
		content, err := ioutil.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		var result = string(content)
		if expand {
			result = context.Expand(result)
			if err != nil {
				return nil, err
			}
		}
		for k, v := range replaceMap {
			result = strings.Replace(result, k, v, len(result))
		}
		return ioutil.NopCloser(strings.NewReader(toolbox.AsString(result))), nil
	}
}

func getStorageService(context *Context, resource *url.Resource) (storage.Service, error) {
	if context.state.Has(UseMemoryService) {
		return storage.NewMemoryService(), nil
	}
	return storage.NewServiceForURL(resource.URL, resource.Credential)
}

//IsShellCompressable returns true if resource can be compress via shell command.
func IsShellCompressable(protScheme string) bool {
	return protScheme == "scp" || protScheme == "file"
}

func (s *transferService) getServiceAndResource(context *Context, resource *url.Resource) (*url.Resource, storage.Service, error) {
	expendedResource, err := context.ExpandResource(resource)
	if err != nil {
		return nil, nil, err
	}
	service, err := getStorageService(context, expendedResource)
	if err != nil {
		return nil, nil, err
	}
	return expendedResource, service, nil
}

func (s *transferService) getModificationHandler(context *Context, transfer *Transfer) func(reader io.ReadCloser) (io.ReadCloser, error) {
	var handler func(reader io.ReadCloser) (io.ReadCloser, error)
	if transfer.Expand || len(transfer.Replace) > 0 {
		handler = NewExpandedContentHandler(context, transfer.Replace, transfer.Expand)
	}
	return handler
}

func (s *transferService) run(context *Context, transfers ...*Transfer) (*TransferCopyResponse, error) {
	var result = &TransferCopyResponse{
		Transferred: make([]*TransferLog, 0),
	}

	for _, transfer := range transfers {

		sourceResource, sourceService, err := s.getServiceAndResource(context, transfer.Source)
		if err != nil {
			return nil, err
		}
		defer sourceService.Close()
		targetResource, targetService, err := s.getServiceAndResource(context, transfer.Target)
		if err != nil {
			return nil, err
		}
		defer targetService.Close()

		var handler = s.getModificationHandler(context, transfer)
		if has, _ := sourceService.Exists(sourceResource.URL); !has {
			return nil, fmt.Errorf("failed to copy: %v %v - Source does not exists", sourceResource.URL, targetResource.URL)
		}

		//TODO add in memory compression for other protocols
		compressed := transfer.Compress && IsShellCompressable(sourceResource.ParsedURL.Scheme) && IsShellCompressable(targetResource.ParsedURL.Scheme)
		var copyEventType = &CopyEventType{
			SourceURL: sourceResource.URL,
			TargetURL: targetResource.URL,
			Expand:    transfer.Expand || len(transfer.Replace) > 0,
		}
		startEvent := s.Begin(context, copyEventType, Pairs("value", copyEventType), Info)
		object, err := sourceService.StorageObject(sourceResource.URL)
		if err != nil {
			return nil, err
		}
		if compressed {
			err = s.compressSource(context, sourceResource, targetResource, object)
			if err != nil {
				return nil, err
			}
		}
		err = storage.Copy(sourceService, sourceResource.URL, targetService, targetResource.URL, handler, nil)
		s.End(context)(startEvent, Pairs())
		if err != nil {
			return result, err
		}
		if compressed {
			err = s.decompressTarget(context, sourceResource, targetResource, object)
			if err != nil {
				return nil, err
			}
		}
		info := NewTransferLog(context, sourceResource.URL, targetResource.URL, err, transfer.Expand)
		result.Transferred = append(result.Transferred, info)
	}
	return result, nil
}

func (s *transferService) compressSource(context *Context, source, target *url.Resource, sourceObject storage.Object) error {
	var baseDirectory, name = path.Split(source.ParsedURL.Path)
	var archiveSource = name

	if sourceObject.IsFolder() {
		baseDirectory = source.DirectoryPath()
		_, name = path.Split(baseDirectory)
		archiveSource = "*"
	}
	var archiveName = fmt.Sprintf("%v.tar.gz", name)
	response, err := context.Execute(source, &CommandRequest{
		Commands: []string{
			fmt.Sprintf("cd %v", baseDirectory),
			fmt.Sprintf("tar cvzf %v %v", archiveName, archiveSource),
		},
		TimeoutMs: CompressionTimeout,
	})

	if err != nil {
		return err
	}
	if CheckNoSuchFileOrDirectory(response.Stdout()) {
		return fmt.Errorf("faied to compress: %v, %v", fmt.Sprintf("tar cvzf %v %v", archiveName, archiveSource), response.Stdout())
	}

	if sourceObject.IsFolder() {
		source.URL = toolbox.URLPathJoin(source.URL, archiveName)
		source.ParsedURL, _ = url2.Parse(source.URL)
		target.URL = toolbox.URLPathJoin(target.URL, archiveName)
		target.ParsedURL, _ = url2.Parse(target.URL)
		return nil
	}

	if err = source.Rename(archiveName); err == nil {
		if path.Ext(target.ParsedURL.Path) != "" {
			err = target.Rename(archiveName)
		} else {
			target.URL = toolbox.URLPathJoin(target.URL, archiveName)
			target.ParsedURL, _ = url2.Parse(target.URL)
		}
	}
	return err
}

func (s *transferService) decompressTarget(context *Context, source, target *url.Resource, sourceObject storage.Object) error {

	var baseDir, name = path.Split(target.ParsedURL.Path)

	_, err := context.Execute(target, &CommandRequest{
		Commands: []string{
			fmt.Sprintf("mkdir -p %v", baseDir),
			fmt.Sprintf("cd %v", baseDir),
		},
	})

	if err == nil {
		_, err = context.Execute(target, &CommandRequest{
			Commands: []string{
				fmt.Sprintf("tar xvzf %v", name),
				fmt.Sprintf("rm %v", name),
			},
			TimeoutMs: CompressionTimeout,
		})
	}
	if err == nil {
		_, err = context.Execute(target, &CommandRequest{
			Commands: []string{
				fmt.Sprintf("cd %v", source.DirectoryPath()),
				fmt.Sprintf("rm %v", name),
			},
		})
	}

	return err
}

func (s *transferService) Run(context *Context, request interface{}) *ServiceResponse {
	startEvent := s.Begin(context, request, Pairs("request", request))
	var response = &ServiceResponse{Status: "ok"}
	defer s.End(context)(startEvent, Pairs("response", response))
	var err error
	switch actualRequest := request.(type) {
	case *TransferCopyRequest:
		response.Response, err = s.run(context, actualRequest.Transfers...)
		if err != nil {
			response.Error = fmt.Sprintf("unable transfer, %v", err)
		}
	default:
		response.Error = fmt.Sprintf("unsupported request type: %T", request)
	}
	if response.Error != "" {
		response.Status = "err"
	}
	return response
}

func (s *transferService) NewRequest(action string) (interface{}, error) {
	switch action {
	case TransferServiceCopyAction:
		return &TransferCopyRequest{
			Transfers: make([]*Transfer, 0),
		}, nil
	}
	return s.AbstractService.NewRequest(action)
}

//NewTransferService creates a new transfer service
func NewTransferService() Service {
	var result = &transferService{
		AbstractService: NewAbstractService(TransferServiceID,
			TransferServiceCopyAction),
	}
	result.AbstractService.Service = result
	return result

}
