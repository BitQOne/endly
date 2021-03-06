package endly

import (
	"fmt"
	"github.com/lunixbochs/vtclean"
	"github.com/viant/toolbox"
	"github.com/viant/toolbox/url"
	"strings"
)

const (
	//DockerServiceID represents docker service id
	DockerServiceID = "docker"

	//DockerServiceRunAction represents docker run action
	DockerServiceRunAction = "run"

	//DockerServiceSysPathAction represents docker syspath action
	DockerServiceSysPathAction = "syspath"

	//DockerServiceStopImagesAction represents docker stop-images" action
	DockerServiceStopImagesAction = "stop-images"

	//DockerServiceImagesAction represents docker images action
	DockerServiceImagesAction = "images"

	//DockerServicePullAction represents docker pull action
	DockerServicePullAction = "pull"

	//DockerServiceContainerCommandAction represents docker container-command action
	DockerServiceContainerCommandAction = "container-command"

	//DockerServiceContainerStartAction represents docker container-start action
	DockerServiceContainerStartAction = "container-start"

	//DockerServiceContainerStopAction represents docker container-stop action
	DockerServiceContainerStopAction = "container-stop"

	//DockerServiceContainerStatusAction represents docker container-status action
	DockerServiceContainerStatusAction = "container-status"

	//DockerServiceContainerRemoveAction represents docker container-remove action
	DockerServiceContainerRemoveAction = "container-remove"

	containerInUse    = "is already in use by container"
	unableToFindImage = "unable to find image"
	dockerError       = "Error response"
)

var dockerErrors = []string{"failed", unableToFindImage}
var dockerIgnoreErrors = []string{}

type dockerService struct {
	*AbstractService
	SysPath []string
}

func (s *dockerService) NewRequest(action string) (interface{}, error) {
	switch action {
	case DockerServiceRunAction:
		return &DockerRunRequest{}, nil
	case DockerServiceSysPathAction:
		return &DockerSystemPathRequest{}, nil
	case DockerServiceStopImagesAction:
		return &DockerStopImagesRequest{}, nil
	case DockerServiceImagesAction:
		return &DockerImagesRequest{}, nil
	case DockerServicePullAction:
		return &DockerPullRequest{}, nil
	case DockerServiceContainerCommandAction:
		return &DockerContainerCommandRequest{}, nil
	case DockerServiceContainerStartAction:
		return &DockerContainerStartRequest{}, nil
	case DockerServiceContainerStopAction:
		return &DockerContainerStopRequest{}, nil
	case DockerServiceContainerStatusAction:
		return &DockerContainerStatusRequest{}, nil
	case DockerServiceContainerRemoveAction:
		return &DockerContainerRemoveRequest{}, nil
	}
	return s.AbstractService.NewRequest(action)
}

func (s *dockerService) Run(context *Context, request interface{}) *ServiceResponse {
	startEvent := s.Begin(context, request, Pairs("request", request))
	s.mutex.Lock()
	defer s.mutex.Unlock()
	var response = &ServiceResponse{Status: "ok"}
	defer s.End(context)(startEvent, Pairs("response", response))
	var err error
	var errorMessage string
	switch actualRequest := request.(type) {

	case *DockerSystemPathRequest:
		s.SysPath = actualRequest.SysPath
	case *DockerImagesRequest:
		response.Response, err = s.checkImages(context, actualRequest)
		errorMessage = fmt.Sprintf("failed to check images %v", actualRequest.Tag)
	case *DockerPullRequest:
		response.Response, err = s.pullImage(context, actualRequest)
		errorMessage = fmt.Sprintf("failed to pull image %v", actualRequest.Tag)
	case *DockerContainerStatusRequest:
		response.Response, err = s.checkContainerProcesses(context, actualRequest)
		errorMessage = "failed to check process"
	case *DockerContainerCommandRequest:
		response.Response, err = s.runInContainer(context, actualRequest)
		errorMessage = fmt.Sprintf("failed to run docker command %v in %v", actualRequest.Command, actualRequest.Target.Name)
	case *DockerContainerStartRequest:
		response.Response, err = s.startContainer(context, actualRequest)
		errorMessage = fmt.Sprintf("failed start container %v", actualRequest.Target.Name)
	case *DockerContainerStopRequest:
		response.Response, err = s.stopContainer(context, actualRequest)
		errorMessage = fmt.Sprintf("failed to stop container: %v", actualRequest.Target.Name)
	case *DockerContainerRemoveRequest:
		response.Response, err = s.removeContainer(context, actualRequest)
		errorMessage = fmt.Sprintf("failed to remove container: %v", actualRequest.Target.Name)
	case *DockerRunRequest:
		response.Response, err = s.runContainer(context, actualRequest)
		errorMessage = fmt.Sprintf("failed to run container: %v", actualRequest.Target.Name)
	case *DockerStopImagesRequest:
		response.Response, err = s.stopImages(context, actualRequest)
		errorMessage = fmt.Sprintf("failed to stop images: %v", actualRequest.Images)
	default:
		err = fmt.Errorf("unsupported request type: %T", request)
	}
	if err != nil {
		response.Status = "error"
		response.Error = errorMessage + ", " + err.Error()
	}
	return response
}

func (s *dockerService) stopImages(context *Context, request *DockerStopImagesRequest) (*DockerStopImagesResponse, error) {
	s.applySysPathIfNeeded(request.SysPath)
	var response = &DockerStopImagesResponse{
		StoppedImages: make([]string, 0),
	}
	processResponse, err := s.checkContainerProcesses(context, &DockerContainerStatusRequest{
		Target:  request.Target,
		SysPath: request.SysPath,
	})
	if err != nil {
		return nil, err
	}

	for _, image := range request.Images {

		for _, container := range processResponse.Containers {
			if strings.Contains(container.Image, image) {
				var containerTarget = request.Target.Clone()
				containerTarget.Name = strings.Split(container.Names, ",")[0]
				_, err = s.stopContainer(context, &DockerContainerStopRequest{
					Target: containerTarget,
				})
				if err != nil {
					return nil, err
				}
				response.StoppedImages = append(response.StoppedImages, container.Image)
			}
		}
	}
	return response, nil
}

/**
	https://docs.docker.com/compose/reference/run/
Options:
    -d                    Detached mode: Run container in the background, print
                          new container Id.
    --Id NAME           Assign a Id to the container
    --entrypoint CMD      Override the entrypoint of the image.
    -e KEY=VAL            Set an environment variable (can be used multiple times)
    -u, --user=""         Run as specified username or uid
    --no-deps             Don't start linked services.
    --rm                  Remove container after run. Ignored in detached mode.
    -p, --publish=[]      Publish a container's port(s) to the host
    --service-ports       Run command with the service's ports enabled and mapped
                          to the host.
    -v, --volume=[]       Bind mount a volume (default [])
    -T                    Disable pseudo-tty allocation. By default `docker-compose run`
                          allocates a TTY.
    -w, --workdir=""      Working directory inside the container

*/

func (s *dockerService) applySysPathIfNeeded(sysPath []string) {
	if len(sysPath) > 0 {
		s.SysPath = sysPath
	}
}

func (s *dockerService) applyCredentialIfNeeded(credentials map[string]string) map[string]string {
	var result = make(map[string]string)
	if len(credentials) > 0 {
		for k, v := range credentials {
			result[k] = v
		}
	}
	return result
}

func (s *dockerService) resetContainerIfNeeded(context *Context, target *url.Resource, statusResponse *DockerContainerStatusResponse) error {
	if len(statusResponse.Containers) > 0 {
		_, err := s.stopContainer(context, &DockerContainerStopRequest{
			Target: target,
		})
		if err != nil {
			return err
		}
		_, err = s.removeContainer(context, &DockerContainerRemoveRequest{Target: target})
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *dockerService) runContainer(context *Context, request *DockerRunRequest) (*DockerContainerInfo, error) {
	var err error
	if err = request.Validate(); err != nil {
		return nil, err
	}
	s.applySysPathIfNeeded(request.SysPath)
	var credentials = s.applyCredentialIfNeeded(request.Credentials)

	checkResponse, err := s.checkContainerProcesses(context, &DockerContainerStatusRequest{
		Target: request.Target,
		Names:  request.Target.Name,
	})
	if err == nil {
		err = s.resetContainerIfNeeded(context, request.Target, checkResponse)
	}
	if err != nil {
		return nil, err
	}
	var args = ""
	for k, v := range request.Env {
		args += fmt.Sprintf("-e %v=%v ", k, context.Expand(v))
	}
	for k, v := range request.Mount {
		args += fmt.Sprintf("-v %v:%v ", context.Expand(k), context.Expand(v))
	}
	for k, v := range request.MappedPort {
		args += fmt.Sprintf("-p %v:%v ", context.Expand(toolbox.AsString(k)), context.Expand(toolbox.AsString(v)))
	}
	if request.Workdir != "" {
		args += fmt.Sprintf("-w %v ", context.Expand(request.Workdir))
	}
	var params = ""
	for k, v := range request.Params {
		params += fmt.Sprintf("%v %v", k, v)
	}
	commandInfo, err := s.executeSecureDockerCommand(credentials, context, request.Target, dockerIgnoreErrors, "docker run --name %v %v -d %v %v", request.Target.Name, args, request.Image, params)
	if err != nil {
		return nil, err
	}

	if strings.Contains(commandInfo.Stdout(), containerInUse) {
		_, _ = s.stopContainer(context, &DockerContainerStopRequest{Target: request.Target})
		_, _ = s.removeContainer(context, &DockerContainerRemoveRequest{Target: request.Target})
		commandInfo, err = s.executeSecureDockerCommand(credentials, context, request.Target, dockerErrors, "docker run --name %v %v -d %v", request.Target.Name, args, request.Image)
		if err != nil {
			return nil, err
		}
	}

	return s.checkContainerProcess(context, &DockerContainerStatusRequest{
		Target:  request.Target,
		Names:   request.Target.Name,
		SysPath: request.SysPath,
	})
}

func (s *dockerService) checkContainerProcess(context *Context, request *DockerContainerStatusRequest) (*DockerContainerInfo, error) {
	s.applySysPathIfNeeded(request.SysPath)
	checkResponse, err := s.checkContainerProcesses(context, request)
	if err != nil {
		return nil, err
	}

	if len(checkResponse.Containers) == 1 {
		return checkResponse.Containers[0], nil
	}
	return nil, nil
}

func (s *dockerService) startContainer(context *Context, request *DockerContainerStartRequest) (*DockerContainerInfo, error) {

	if request.Target.Name == "" {
		return nil, fmt.Errorf("target name was empty url: %v", request.Target.URL)
	}
	s.applySysPathIfNeeded(request.SysPath)
	_, err := s.executeDockerCommand(nil, context, request.Target, dockerErrors, "docker start %v", request.Target.Name)
	if err != nil {
		return nil, err
	}
	return s.checkContainerProcess(context, &DockerContainerStatusRequest{
		Target:  request.Target,
		Names:   request.Target.Name,
		SysPath: request.SysPath,
	})

}

func (s *dockerService) stopContainer(context *Context, request *DockerContainerStopRequest) (*DockerContainerInfo, error) {
	if request.Target.Name == "" {
		return nil, fmt.Errorf("target name was empty for %v", request.Target.URL)
	}
	s.applySysPathIfNeeded(request.SysPath)
	info, err := s.checkContainerProcess(context, &DockerContainerStatusRequest{
		Target:  request.Target,
		Names:   request.Target.Name,
		SysPath: request.SysPath,
	})
	if err != nil {
		return nil, err
	}
	if info == nil {
		return nil, err
	}

	_, err = s.executeDockerCommand(nil, context, request.Target, dockerErrors, "docker stop %v", request.Target.Name)
	if err != nil {
		return nil, err
	}
	if info != nil {
		info.Status = "down"
	}
	return info, nil
}

func (s *dockerService) removeContainer(context *Context, request *DockerContainerRemoveRequest) (*CommandResponse, error) {
	if request.Target.Name == "" {
		return nil, fmt.Errorf("Target name was empty for %v", request.Target.URL)
	}
	s.applySysPathIfNeeded(request.SysPath)
	commandInfo, err := s.executeDockerCommand(nil, context, request.Target, dockerErrors, "docker rm %v", request.Target.Name)
	if err != nil {
		return nil, err
	}
	return commandInfo, nil
}

func (s *dockerService) runInContainer(context *Context, request *DockerContainerCommandRequest) (*CommandResponse, error) {
	if request.Target.Name == "" {
		return nil, fmt.Errorf("Target name was empty for %v and command %v", request.Target.URL, request.Command)
	}
	s.applySysPathIfNeeded(request.SysPath)

	var executionOptions = ""
	var command = context.Expand(request.Command)

	if request.Interactive {
		executionOptions += "i"
	}
	if request.AllocateTerminal {
		executionOptions += "t"
	}
	if request.RunInTheBackground {
		executionOptions += "d"
	}
	if executionOptions != "" {
		executionOptions = "-" + executionOptions
	}

	commandRespons, err := s.executeSecureDockerCommand(request.Credentials, context, request.Target, dockerErrors, "docker exec %v %v %v", executionOptions, request.Target.Name, command)
	if err != nil {
		return nil, err
	}
	if len(commandRespons.Commands) > 1 {
		//Truncate password auth, to process vanila container output
		var stdout = commandRespons.Commands[0].Stdout
		if strings.Contains(stdout, "Password:") {
			commandRespons.Commands = commandRespons.Commands[1:]
		}
	}
	return commandRespons, err
}

func (s *dockerService) checkContainerProcesses(context *Context, request *DockerContainerStatusRequest) (*DockerContainerStatusResponse, error) {
	s.applySysPathIfNeeded(request.SysPath)
	info, err := s.executeSecureDockerCommand(nil, context, request.Target, dockerErrors, "docker ps")
	if err != nil {
		return nil, err
	}
	stdout := info.Stdout()

	var containers = make([]*DockerContainerInfo, 0)
	var lines = strings.Split(stdout, "\r\n")
	for i := 1; i < len(lines); i++ {
		columns, ok := ExtractColumns(lines[i])
		if !ok || len(columns) < 7 {
			continue
		}
		var status = "down"
		if strings.Contains(lines[i], "Up") {
			status = "up"
		}
		info := &DockerContainerInfo{
			ContainerID: columns[0],
			Image:       columns[1],
			Command:     strings.Trim(columns[2], "\""),
			Status:      status,
			Port:        columns[len(columns)-2],
			Names:       columns[len(columns)-1],
		}
		if request.Image != "" && request.Image != info.Image {
			continue
		}
		if request.Names != "" && request.Names != info.Names {
			continue
		}
		containers = append(containers, info)
	}
	return &DockerContainerStatusResponse{Containers: containers}, nil
}

func (s *dockerService) pullImage(context *Context, request *DockerPullRequest) (*DockerImageInfo, error) {
	if request.Tag == "" {
		request.Tag = "latest"
	}
	info, err := s.executeDockerCommand(nil, context, request.Target, dockerErrors, "docker pull %v:%v", request.Repository, request.Tag)
	if err != nil {
		return nil, err
	}
	stdout := info.Stdout()
	if strings.Contains(stdout, "not found") {
		return nil, fmt.Errorf("failed to pull docker image,  %v", stdout)
	}
	imageResponse, err := s.checkImages(context, &DockerImagesRequest{Target: request.Target, Repository: request.Repository, Tag: request.Tag})
	if err != nil {
		return nil, err
	}
	if len(imageResponse.Images) == 1 {
		return imageResponse.Images[0], nil
	}
	return nil, fmt.Errorf("failed to check image status: %v:%v found: %v", request.Repository, request.Tag, len(imageResponse.Images))
}

func (s *dockerService) checkImages(context *Context, request *DockerImagesRequest) (*DockerImagesResponse, error) {
	if request.SysPath != nil {
		s.SysPath = request.SysPath
	}

	info, err := s.executeDockerCommand(nil, context, request.Target, dockerErrors, "docker images")
	if err != nil {
		return nil, err
	}
	stdout := info.Stdout()
	var images = make([]*DockerImageInfo, 0)

	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "REPOSITORY") {
			continue
		}
		columns, ok := ExtractColumns(line)
		if !ok || len(columns) < 4 {
			continue
		}

		var size = ""
		for j := len(columns) - 2; j < len(columns); j++ {
			if strings.Contains(columns[j], "ago") {
				continue
			}
			size += columns[j]
		}
		var sizeFactor = 1024
		var sizeUnit = "KB"
		if strings.Contains(line, "MB") {
			sizeUnit = "MB"
			sizeFactor *= 1024
		}

		size = strings.Replace(size, sizeUnit, "", 1)
		info := &DockerImageInfo{
			Repository: columns[0],
			Tag:        columns[1],
			ImageID:    columns[2],
			Size:       toolbox.AsInt(size) * sizeFactor,
		}

		if request.Repository != "" {
			if info.Repository != request.Repository {
				continue
			}
		}

		if request.Tag != "" {
			if info.Tag != request.Tag {
				continue
			}
		}
		images = append(images, info)
	}
	return &DockerImagesResponse{Images: images}, nil

}

func (s *dockerService) executeDockerCommand(secure map[string]string, context *Context, target *url.Resource, errors []string, template string, arguments ...interface{}) (*CommandResponse, error) {
	return s.executeSecureDockerCommand(secure, context, target, errors, template, arguments...)
}

func (s *dockerService) executeSecureDockerCommand(secure map[string]string, context *Context, target *url.Resource, errors []string, template string, arguments ...interface{}) (*CommandResponse, error) {
	command := fmt.Sprintf(template, arguments...)
	if len(secure) == 0 {
		secure = make(map[string]string)
	}
	secure[sudoCredentialKey] = target.Credential
	response, err := context.Execute(target, &superUserCommandRequest{
		MangedCommand: &ExtractableCommand{
			Options: &ExecutionOptions{
				SystemPaths: s.SysPath,
			},
			Executions: []*Execution{
				{
					Credentials: secure,
					Command:     command,
					Error:       append(errors, []string{commandNotFound}...),
				},
			},
		},
	})

	if err != nil {
		return nil, err
	}
	var stdout = response.Stdout()
	if strings.Contains(stdout, containerInUse) {
		return response, err
	}
	if strings.Contains(stdout, dockerError) {
		return response, fmt.Errorf("Error executing %v, %v", command, vtclean.Clean(stdout, false))
	}
	return response, nil
}

//NewDockerService returns a new docker service.
func NewDockerService() Service {
	var result = &dockerService{
		AbstractService: NewAbstractService(DockerServiceID,
			DockerServiceRunAction,
			DockerServiceSysPathAction,
			DockerServiceStopImagesAction,
			DockerServiceImagesAction,
			DockerServicePullAction,
			DockerServiceContainerCommandAction,
			DockerServiceContainerStartAction,
			DockerServiceContainerStopAction,
			DockerServiceContainerStatusAction,
			DockerServiceContainerRemoveAction,
		),
	}
	result.AbstractService.Service = result
	return result
}

//DockerSystemPathRequest represents system path request to set docker in the path
type DockerSystemPathRequest struct {
	SysPath []string
}
