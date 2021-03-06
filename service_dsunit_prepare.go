package endly

import (
	"fmt"
	"github.com/viant/dsunit"
)

//DsUnitPrepareRequest represents a dsunit prepare requests.
type DsUnitPrepareRequest struct {
	Datastore  string                              // name of registered datastore
	URL        string                              //if URL is provided then all files listed from the path are setup data candidates
	Credential string                              // optional URL credential
	Prefix     string                              //apply prefix
	Postfix    string                              //apply suffix
	Data       map[string][]map[string]interface{} //setup data, where the first map key is table name with value being records
	Expand     bool                                //substitute dollar($) expression with the state map
}

//DsUnitPrepareResponse represents dsunit prepare response.
type DsUnitPrepareResponse struct {
	Added    int
	Modified int
	Deleted  int
}

//Validate checks if request is valid
func (r *DsUnitPrepareRequest) Validate() error {
	if r.Datastore == "" {
		return fmt.Errorf("Datasets.Datastore was empty")
	}
	if r.URL == "" && len(r.Data) == 0 {
		return fmt.Errorf("Missing data: Datasets.URL/Datasets.TableRows were empty")
	}
	return nil
}

//AsDatasetResource converts request as *dsunit.DatasetResource
func (r *DsUnitPrepareRequest) AsDatasetResource() *dsunit.DatasetResource {
	var result = &dsunit.DatasetResource{
		Datastore:  r.Datastore,
		URL:        r.URL,
		Credential: r.Credential,
		Prefix:     r.Prefix,
		Postfix:    r.Postfix,
	}
	if len(r.Data) > 0 {
		result.TableRows = make([]*dsunit.TableRows, 0)
		for table, data := range r.Data {
			var tableRows = &dsunit.TableRows{
				Table: table,
				Rows:  data,
			}
			result.TableRows = append(result.TableRows, tableRows)
		}
	}
	return result
}
