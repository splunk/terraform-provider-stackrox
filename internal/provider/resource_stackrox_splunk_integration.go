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
	"fmt"
	"net/http"
	"strconv"

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

func resourceStackRoxSplunkIntegration() *schema.Resource {
	return &schema.Resource{
		Create:   stackRoxSplunkIntegrationCreate,
		Read:     stackRoxSplunkIntegrationRead,
		Update:   stackRoxSplunkIntegrationUpdate,
		Delete:   stackRoxSplunkIntegrationDelete,
		Importer: stackRoxSplunkIntegrationImporter(),
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"hec_endpoint": {
				Type:     schema.TypeString,
				Required: true,
			},
			"hec_token": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},
			"truncate": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  10000,
			},
			"ui_endpoint": {
				Type:     schema.TypeString,
				Required: true,
			},
			"audit_logging_enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
}

func stackRoxSplunkIntegrationCreate(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxSplunkIntegrationCreate")

	message := stackrox.StorageNotifier{
		Name:       data.Get("name").(string),
		UiEndpoint: data.Get("ui_endpoint").(string),
		Type:       "splunk",
		Splunk: &stackrox.StorageSplunk{
			HttpEndpoint:        data.Get("hec_endpoint").(string),
			HttpToken:           data.Get("hec_token").(string),
			Insecure:            false,
			Truncate:            strconv.Itoa(data.Get("truncate").(int)),
			AuditLoggingEnabled: data.Get("audit_logging_enabled").(bool),
		},
		Enabled: true,
	}

	logMessage(message)

	cli := meta.(ClientWrap)
	result, resp, err := cli.NotifierServiceApi.PostNotifier(cli.BasicAuthContext(), message)
	logResult(result, resp, err)
	if err != nil {
		return err
	}

	// Set the ID of the resource to the id. A non-blank ID
	// tells Terraform that a resource was created.
	data.SetId(result.Id)
	return stackRoxSplunkIntegrationRead(data, meta)
}

func stackRoxSplunkIntegrationRead(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxSplunkIntegrationRead")

	// Attempt to read from an upstream API.
	cli := meta.(ClientWrap)
	result, resp, err := cli.NotifierServiceApi.GetNotifier(cli.BasicAuthContext(), data.Id())
	logResult(result, resp, err)

	// If the resource does not exist, inform Terraform. We want to immediately
	// return here to prevent further processing.
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		data.SetId("")
		return nil
	}

	// Update the local state.
	stackRoxSplunkIntegrationSetState(data, result)
	return nil
}

func stackRoxSplunkIntegrationUpdate(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxSplunkIntegrationUpdate")

	return stackRoxSplunkIntegrationRead(data, meta)
}

func stackRoxSplunkIntegrationDelete(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxSplunkIntegrationDelete: " + data.Id())

	// Attempt to delete from an upstream API.
	// data.SetId("") is automatically called assuming delete returns no errors.
	cli := meta.(ClientWrap)
	result, resp, err := cli.NotifierServiceApi.DeleteNotifier(cli.BasicAuthContext(), data.Id(), &stackrox.DeleteNotifierOpts{Force: optional.NewBool(true)})
	logResult(result, resp, err)

	// Destroy should be idempotent. The cluster API returns 404 when the resource isn't found.
	if resp != nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	return fmt.Errorf(resp.Status)
}

func stackRoxSplunkIntegrationImporter() *schema.ResourceImporter {
	return &schema.ResourceImporter{
		State: stackRoxSplunkIntegrationImportState,
	}
}

func stackRoxSplunkIntegrationImportState(data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	debug("calling stackRoxSplunkIntegrationImportState")

	// Attempt to read from an upstream API.
	cli := meta.(ClientWrap)
	id := data.Id()
	result, resp, err := cli.NotifierServiceApi.GetNotifier(cli.BasicAuthContext(), id)
	logResult(result, resp, err)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(resp.Status)
	}

	// Import the resource.
	newData := &schema.ResourceData{}
	newData.SetType("stackrox_splunk_integration")
	newData.SetId(id)

	if err := stackRoxSplunkIntegrationSetState(newData, result); err != nil {
		return nil, err
	}

	return []*schema.ResourceData{newData}, nil
}

func stackRoxSplunkIntegrationSetState(data *schema.ResourceData, src stackrox.StorageNotifier) error {
	truncate, err := strconv.Atoi(src.Splunk.Truncate)
	if err != nil {
		return err
	}

	data.Set("name", src.Name)
	data.Set("hec_endpoint", src.Splunk.HttpEndpoint)
	data.Set("truncate", truncate)
	data.Set("ui_endpoint", src.UiEndpoint)
	data.Set("audit_logging_enabled", src.Splunk.AuditLoggingEnabled)
	// The `hec_token` isn't returned by the API. So, the state is left alone.

	return nil
}
