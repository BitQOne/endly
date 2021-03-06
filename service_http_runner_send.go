package endly

import (
	"github.com/viant/toolbox"
	"net/http"
)

//SendHTTPRequest represents a send http request.
type SendHTTPRequest struct {
	Options     []*toolbox.HttpOptions
	Requests    []*HTTPRequest
}

//HTTPRequest represents an http request
type HTTPRequest struct {
	MatchBody    string //only run this execution is output from a previous command is matched
	Method       string
	URL          string
	Header       http.Header
	Cookies      Cookies
	Body         string
	Replace      map[string]string //replaces key with value if present
	Extraction   DataExtractions   //extraction
	Variables    Variables         // input JSON body map, output state.httpPrevious
	Repeat       int               //how many time send this request
	SleepTimeMs  int               //Sleep time after request send, this only makes sense with repeat option
	ExitCriteria string            //Repeat exit criteria, it uses extracted variable to determine repeat termination
	RequestUdf  string
	ResponseUdf string
}

//SendHTTPResponse represnets a send response
type SendHTTPResponse struct {
	Responses []*HTTPResponse
	Extracted map[string]string
}

//HTTPResponse represents Http response
type HTTPResponse struct {
	//Request     *HTTPRequest
	Code        int
	Header      http.Header
	Cookies     map[string]*http.Cookie
	Body        string
	JSONBody    map[string]interface{}
	TimeTakenMs int
	Error       string
}

//Cookies represents cookie
type Cookies []*http.Cookie

//SetHeader sets cookie header
func (c *Cookies) SetHeader(header http.Header) {
	if len(*c) == 0 {
		return
	}
	for _, cookie := range *c {
		if v := cookie.String(); v != "" {
			header.Add("Cookie", v)
		}
	}
}

//IndexByName index cookie by name
func (c *Cookies) IndexByName() map[string]*http.Cookie {
	var result = make(map[string]*http.Cookie)
	for _, cookie := range *c {
		result[cookie.Name] = cookie
	}
	return result
}

//IndexByPosition index cookie by position
func (c *Cookies) IndexByPosition() map[string]int {
	var result = make(map[string]int)
	for i, cookie := range *c {
		result[cookie.Name] = i
	}
	return result
}

//AddCookies adds cookies
func (c *Cookies) AddCookies(cookies ...*http.Cookie) {
	if len(cookies) == 0 {
		return
	}
	var indexed = c.IndexByPosition()
	for _, cookie := range cookies {

		if position, has := indexed[cookie.Name]; has {
			(*c)[position] = cookie
		} else {
			*c = append(*c, cookie)
		}
	}
}

//Expand substitute request data with matching context map state.
func (r *HTTPRequest) Expand(context *Context) *HTTPRequest {
	header := make(map[string][]string)
	copyExpandedHeaders(r.Header, header, context)
	return &HTTPRequest{
		MatchBody:    context.Expand(r.MatchBody),
		Method:       r.Method,
		URL:          context.Expand(r.URL),
		Body:         context.Expand(r.Body),
		Header:       header,
		Extraction:   r.Extraction,
		Variables:    r.Variables,
		Replace:      r.Replace,
		Repeat:       r.Repeat,
		SleepTimeMs:  r.SleepTimeMs,
		ExitCriteria: r.ExitCriteria,
		RequestUdf:   r.RequestUdf,
		ResponseUdf:  r.ResponseUdf,
	}
}
