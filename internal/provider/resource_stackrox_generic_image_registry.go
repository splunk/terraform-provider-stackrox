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

	"github.com/antihax/optional"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

func resourceStackRoxGenericImageRegistry() *schema.Resource {
	return &schema.Resource{
		Create:   stackRoxGenericImageRegistryCreate,
		Read:     stackRoxGenericImageRegistryRead,
		Update:   stackRoxGenericImageRegistryUpdate,
		Delete:   stackRoxGenericImageRegistryDelete,
		Importer: stackRoxGenericImageRegistryImporter(),
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"endpoint": {
				Type:     schema.TypeString,
				Required: true,
			},
			"username": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"password": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
		},
	}
}

func stackRoxGenericImageRegistryCreate(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxGenericImageRegistryCreate")

	message := stackrox.StorageImageIntegration{
		Name:       data.Get("name").(string),
		Categories: []stackrox.StorageImageIntegrationCategory{stackrox.STORAGEIMAGEINTEGRATIONCATEGORY_REGISTRY},
		Type:       "docker",
		Docker: &stackrox.StorageDockerConfig{
			Endpoint: data.Get("endpoint").(string),
			Username: data.Get("username").(string),
			Password: data.Get("password").(string),
			Insecure: false,
		},
		SkipTestIntegration: true,
	}

	logMessage(message)

	cli := meta.(ClientWrap)
	result, resp, err := cli.ImageIntegrationServiceApi.PostImageIntegration(cli.BasicAuthContext(), message)
	logResult(result, resp, err)
	if err != nil {
		return err
	}

	data.SetId(result.Id)
	return stackRoxGenericImageRegistryRead(data, meta)
}

func stackRoxGenericImageRegistryRead(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxGenericImageRegistryRead")

	// Attempt to read from an upstream API.
	cli := meta.(ClientWrap)
	result, resp, err := cli.ImageIntegrationServiceApi.GetImageIntegration(cli.BasicAuthContext(), data.Id())
	logResult(result, resp, err)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(resp.Status)
	}

	// If the resource does not exist, inform Terraform. We want to immediately
	// return here to prevent further processing.
	if resp.StatusCode == http.StatusNotFound {
		data.SetId("")
		return nil
	}

	// Update the local state.
	stackRoxGenericImageRegistrySetState(data, result)
	return nil
}

func stackRoxGenericImageRegistryUpdate(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxGenericImageRegistryUpdate")

	return stackRoxGenericImageRegistryRead(data, meta)
}

func stackRoxGenericImageRegistryDelete(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxGenericImageRegistryDelete: " + data.Id())

	// Attempt to delete from an upstream API.
	// data.SetId("") is automatically called assuming delete returns no errors.
	cli := meta.(ClientWrap)
	result, resp, err := cli.ImageIntegrationServiceApi.DeleteImageIntegration(cli.BasicAuthContext(), data.Id())
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

func stackRoxGenericImageRegistryImporter() *schema.ResourceImporter {
	return &schema.ResourceImporter{
		State: stackRoxGenericImageRegistryImportState,
	}
}

// Import by name.
func stackRoxGenericImageRegistryImportState(data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	debug("calling stackRoxGenericImageRegistryImportState")

	// Attempt to read from an upstream API, using the name as a natural key.
	cli := meta.(ClientWrap)
	result, resp, err := cli.ImageIntegrationServiceApi.GetImageIntegrations(
		cli.BasicAuthContext(),
		&stackrox.GetImageIntegrationsOpts{
			Name: optional.NewString(data.Id()),
		},
	)
	logResult(result, resp, err)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(resp.Status)
	}

	if len(result.Integrations) != 1 {
		return nil, fmt.Errorf("invalid number of integrations: %d", len(result.Integrations))
	}

	// Import the resource.
	newData := &schema.ResourceData{}
	newData.SetType("stackrox_generic_image_registry")
	newData.SetId(result.Integrations[0].Id)
	stackRoxGenericImageRegistrySetState(newData, result.Integrations[0])
	// Override the new state with the original state for the `password` field.
	newData.Set("password", data.Get("password"))

	return []*schema.ResourceData{newData}, nil
}

func stackRoxGenericImageRegistrySetState(data *schema.ResourceData, src stackrox.StorageImageIntegration) {
	data.Set("name", src.Name)
	data.Set("endpoint", src.Docker.Endpoint)
	data.Set("username", src.Docker.Username)
	// The `password` isn't returned by the API. So, the state is left alone.
}
