/*
   Copyright 2021 Splunk Inc.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

// FindGroups is a local extension of the generated API, and it provides missing functionality that returns
// a slice of authentication groups filtered by the given authentication provider ID.
func (c ClientWrap) FindGroups(ctx context.Context, authProviderId string) ([]stackrox.StorageGroup, *http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.endpoint+"/v1/groups", nil)
	if err != nil {
		return nil, nil, err
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, resp, err
	}

	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		return nil, resp, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, resp, fmt.Errorf(resp.Status)
	}

	type groupsType struct {
		Groups []stackrox.StorageGroup `json:"groups"`
	}

	groups := groupsType{}

	if err := json.Unmarshal(body, &groups); err != nil {
		return nil, resp, err
	}

	count := 0
	for _, g := range groups.Groups {
		if g.Props.AuthProviderId == authProviderId {
			count++
		}
	}

	result := make([]stackrox.StorageGroup, 0, count)
	for _, g := range groups.Groups {
		if g.Props.AuthProviderId == authProviderId {
			result = append(result, g)
		}
	}

	return result, resp, nil
}
