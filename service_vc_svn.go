package endly

import (
	"fmt"
	"github.com/viant/toolbox/url"
	"path"
	"strings"
)

type svnService struct{}

func (s *svnService) checkInfo(context *Context, request *VcStatusRequest) (*VcInfo, error) {
	target, err := context.ExpandResource(request.Target)
	if err != nil {
		return nil, err
	}
	var result = &VcInfo{}
	response, err := context.Execute(request.Target, &ExtractableCommand{
		Executions: []*Execution{
			{
				Command: fmt.Sprintf("cd %v", target.ParsedURL.Path),
			},
			{
				Command: fmt.Sprintf("svn info"),
				Extraction: []*DataExtraction{
					{
						RegExpr: "^URL:[\\t\\s]+([^\\s]+)",
						Key:     "origin",
					},
					{
						RegExpr: "Revision:\\s+([^\\s]+)",
						Key:     "revision",
					},
				},
			},
			{
				Command: fmt.Sprintf("svn stat"),
			},
		},
	})

	if err != nil {
		return nil, err
	}
	if revison, has := response.Extracted["revision"]; has {
		result.Revision = revison
	}
	if origin, has := response.Extracted["origin"]; has {
		result.Origin = origin
		_, result.Branch = path.Split(origin)
	}
	if strings.Contains(response.Stdout(1), "is not a working copy") {
		return result, nil
	}
	result.IsVersionControlManaged = true

	readSvnStatus(response, result)
	return result, nil
}

func readSvnStatus(commandResult *CommandResponse, response *VcInfo) {
	response.New = make([]string, 0)
	response.Modified = make([]string, 0)
	response.Deleted = make([]string, 0)
	response.Untracked = make([]string, 0)
	for _, line := range strings.Split(commandResult.Stdout(), "\n") {
		if len(line) == 0 {
			continue
		}
		columns, ok := ExtractColumns(line)
		if !ok || len(columns) < 2 {
			continue
		}
		file := columns[1]
		switch columns[0] {
		case "?":
			response.Untracked = append(response.Untracked, file)
		case "A":
			response.New = append(response.New, file)
		case "D":
			response.Deleted = append(response.Deleted, file)
		case "M":
			response.Modified = append(response.Modified, file)
		}
	}
	if len(response.Modified)+len(response.Deleted)+len(response.New) == 0 {
		response.IsUptoDate = true
	}
}

func (s *svnService) pull(context *Context, request *VcPullRequest) (*VcInfo, error) {
	target, err := context.ExpandResource(request.Target)
	if err != nil {
		return nil, err
	}
	return s.runSecureSvnCommand(context, target, request.Origin, "up")
}

func (s *svnService) checkout(context *Context, request *VcCheckoutRequest) (*VcInfo, error) {
	target, err := context.ExpandResource(request.Target)
	if err != nil {
		return nil, err
	}
	return s.runSecureSvnCommand(context, target, request.Origin, "co", request.Origin.URL, target.DirectoryPath())
}

func (s *svnService) runSecureSvnCommand(context *Context, target *url.Resource, origin *url.Resource, command string, arguments ...string) (*VcInfo, error) {
	username, _, err := origin.LoadCredential(true)
	if err != nil {
		return nil, err
	}
	var credentials = make(map[string]string)
	credentials[versionControlCredentialKey] = origin.Credential
	_, err = context.Execute(target, &ExtractableCommand{
		Options: &ExecutionOptions{
			TimeoutMs:   1000 * 200,
			Terminators: []string{"Password for", "(yes/no)?"},
		},
		Executions: []*Execution{
			{
				Command: fmt.Sprintf("svn %v --username=%v %v", command, username, strings.Join(arguments, " ")),
				Error:   []string{"No such file or directory", "Event not found", "Error validating server certificate", "Unable to connect to a repository"},
			},
			{
				Credentials: credentials,
				MatchOutput: "Password",
				Command:     versionControlCredentialKey,
				Error:       []string{"No such file or directory", "Event not found", "Username:"},
			},
			{
				MatchOutput: "Store password unencrypted",
				Command:     "no",
				Error:       []string{"No such file or directory", "Event not found", "Error validating server certificate"},
			},
		},
	})
	err = checkVersionControlAuthErrors(err, origin)
	if err != nil {
		return nil, err
	}
	return s.checkInfo(context, &VcStatusRequest{
		Target: target,
	})
}

func (s *svnService) commit(context *Context, request *VcCommitRequest) (*VcInfo, error) {

	response, err := context.Execute(request.Target, &ExtractableCommand{
		Executions: []*Execution{
			{
				Command: fmt.Sprintf("svn ci -m \"%v\" ", strings.Replace(request.Message, "\"", "'", len(request.Message))),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if CheckNoSuchFileOrDirectory(response.Stdout()) {
		return nil, fmt.Errorf("failed to commit %v", response.Stdout())
	}
	return s.checkInfo(context, &VcStatusRequest{
		Target: request.Target,
	})
}
