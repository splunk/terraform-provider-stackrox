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
	"net/http"

	"github.com/hashicorp/go-cleanhttp"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

func newStackRoxClient(endpoint string) *stackrox.APIClient {
	cfg := stackrox.NewConfiguration()
	cfg.BasePath = endpoint
	return stackrox.NewAPIClient(cfg)
}

// ClientWrap holds the API Client and a context initialized with the Bearer token for accessing the API.
type ClientWrap struct {
	*stackrox.APIClient
	HTTPClient *http.Client
	endpoint   string
	username   string
	password   string
}

func (c ClientWrap) BasicAuthContext() context.Context {
	basicAuth := stackrox.BasicAuth{
		UserName: c.username,
		Password: c.password,
	}

	return context.WithValue(context.Background(), stackrox.ContextBasicAuth, basicAuth)
}

func NewClientWrap(endpoint, username, password string) ClientWrap {
	return ClientWrap{
		APIClient:  newStackRoxClient(endpoint),
		HTTPClient: cleanhttp.DefaultClient(),
		endpoint:   endpoint,
		username:   username,
		password:   password,
	}
}
