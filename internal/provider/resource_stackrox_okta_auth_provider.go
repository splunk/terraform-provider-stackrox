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

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

func resourceStackRoxOktaAuthProvider() *schema.Resource {
	return &schema.Resource{
		Create:   stackRoxOktaAuthProviderCreate,
		Read:     stackRoxOktaAuthProviderRead,
		Update:   stackRoxOktaAuthProviderUpdate,
		Delete:   stackRoxOktaAuthProviderDelete,
		Importer: stackRoxOktaAuthProviderImporter(),
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
			},
			"ui_endpoint": {
				Type:     schema.TypeString,
				Required: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Required: true,
			},
			"idp_metadata_url": {
				Type:      schema.TypeString,
				Sensitive: true,
				Required:  true,
			},
			"sp_issuer": {
				Type:     schema.TypeString,
				Required: true,
			},
			"group": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"key": {
							Type:     schema.TypeString,
							Optional: true,
						},

						"value": {
							Type:     schema.TypeString,
							Optional: true,
						},

						"role": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func stackRoxOktaAuthProviderCreate(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxOktaAuthProviderCreate")

	data.Partial(true)

	if data.HasChanges("name", "type", "ui_endpoint", "enabled", "idp_metadata_url", "sp_issuer") {
		message := stackrox.StorageAuthProvider{
			Name:       data.Get("name").(string),
			Type:       data.Get("type").(string),
			UiEndpoint: data.Get("ui_endpoint").(string),
			Enabled:    data.Get("enabled").(bool),
			Config: map[string]string{
				"idp_metadata_url": data.Get("idp_metadata_url").(string),
				"sp_issuer":        data.Get("sp_issuer").(string),
			},
		}

		logMessage(message)

		cli := meta.(ClientWrap)
		result, resp, err := cli.AuthProviderServiceApi.PostAuthProvider(cli.BasicAuthContext(), message)
		logResult(result, resp, err)
		if err != nil {
			return err
		}

		data.SetId(result.Id)
	}

	groups := data.Get("group").([]interface{})

	requiredGroups := make([]stackrox.StorageGroup, 0, len(groups))
	for _, g := range groups {
		group := g.(map[string]interface{})
		message := stackrox.StorageGroup{
			RoleName: group["role"].(string),
			Props: stackrox.StorageGroupProperties{
				AuthProviderId: data.Id(),
				Key:            group["key"].(string),
				Value:          group["value"].(string),
			},
		}
		requiredGroups = append(requiredGroups, message)
	}

	message := stackrox.V1GroupBatchUpdateRequest{
		RequiredGroups: requiredGroups,
	}
	logMessage(message)

	cli := meta.(ClientWrap)
	result, resp, err := cli.GroupServiceApi.BatchUpdate(cli.BasicAuthContext(), message)
	logResult(result, resp, err)
	if err != nil {
		return err
	}

	data.Partial(false)

	return stackRoxOktaAuthProviderRead(data, meta)
}

func stackRoxOktaAuthProviderRead(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxOktaAuthProviderRead")

	// Attempt to read from an upstream API.
	cli := meta.(ClientWrap)
	authProvider, resp, err := cli.AuthProviderServiceApi.GetAuthProvider(cli.BasicAuthContext(), data.Id())
	logResult(authProvider, resp, err)

	// If the resource does not exist, inform Terraform. We want to immediately
	// return here to prevent further processing.
	if resp.StatusCode == http.StatusNotFound {
		data.SetId("")
		return nil
	}

	if err != nil {
		return err
	}

	groups, resp, err := cli.FindGroups(cli.BasicAuthContext(), data.Id())
	logResult(authProvider, resp, err)

	if err != nil {
		return err
	}

	// Update the local state.
	return stackRoxOktaAuthProviderSetState(data, authProvider, groups)
}

func stackRoxOktaAuthProviderSetState(data *schema.ResourceData, provider stackrox.StorageAuthProvider, groups []stackrox.StorageGroup) error {
	if err := data.Set("name", provider.Name); err != nil {
		return err
	}
	if err := data.Set("type", provider.Type); err != nil {
		return err
	}
	if err := data.Set("ui_endpoint", provider.UiEndpoint); err != nil {
		return err
	}
	if err := data.Set("enabled", provider.Enabled); err != nil {
		return err
	}
	if err := data.Set("idp_metadata_url", provider.Config["idp_metadata_url"]); err != nil {
		return err
	}
	if err := data.Set("sp_issuer", provider.Config["sp_issuer"]); err != nil {
		return err
	}
	groupsData := make([]map[string]interface{}, 0, len(groups))
	for _, g := range groups {
		groupMap := map[string]interface{}{
			"key":   g.Props.Key,
			"value": g.Props.Value,
			"role":  g.RoleName,
		}
		groupsData = append(groupsData, groupMap)
	}
	if err := data.Set("group", groupsData); err != nil {
		return err
	}

	return nil
}

func stackRoxOktaAuthProviderUpdate(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxOktaAuthProviderUpdate")

	data.Partial(true)

	if data.HasChanges("name", "type", "ui_endpoint", "enabled", "idp_metadata_url", "sp_issuer") {
		message := stackrox.StorageAuthProvider{
			Name:       data.Get("name").(string),
			Type:       data.Get("type").(string),
			UiEndpoint: data.Get("ui_endpoint").(string),
			Enabled:    data.Get("enabled").(bool),
			Config: map[string]string{
				"idp_metadata_url": data.Get("idp_metadata_url").(string),
				"sp_issuer":        data.Get("sp_issuer").(string),
			},
		}

		logMessage(message)

		cli := meta.(ClientWrap)
		result, resp, err := cli.AuthProviderServiceApi.PutAuthProvider(cli.BasicAuthContext(), data.Id(), message)
		logResult(result, resp, err)
		if err != nil {
			return err
		}

		data.SetId(result.Id)
	}

	groups := data.Get("group").([]interface{})

	requiredGroups := make([]stackrox.StorageGroup, 0, len(groups))
	for _, g := range groups {
		group := g.(map[string]interface{})
		message := stackrox.StorageGroup{
			RoleName: group["role"].(string),
			Props: stackrox.StorageGroupProperties{
				AuthProviderId: data.Id(),
				Key:            group["key"].(string),
				Value:          group["value"].(string),
			},
		}
		requiredGroups = append(requiredGroups, message)
	}

	message := stackrox.V1GroupBatchUpdateRequest{
		RequiredGroups: requiredGroups,
	}
	logMessage(message)

	cli := meta.(ClientWrap)
	result, resp, err := cli.GroupServiceApi.BatchUpdate(cli.BasicAuthContext(), message)
	logResult(result, resp, err)
	if err != nil {
		return err
	}

	data.Partial(false)

	return stackRoxOktaAuthProviderRead(data, meta)
}

func stackRoxOktaAuthProviderDelete(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxOktaAuthProviderDelete: " + data.Id())

	// Attempt to read from an upstream API.
	cli := meta.(ClientWrap)
	result, resp, err := cli.AuthProviderServiceApi.DeleteAuthProvider(cli.BasicAuthContext(), data.Id())
	logResult(result, resp, err)

	return err
}

func stackRoxOktaAuthProviderImporter() *schema.ResourceImporter {
	return &schema.ResourceImporter{
		State: stackRoxOktaAuthProviderImportState,
	}
}

func stackRoxOktaAuthProviderImportState(data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	debug("calling stackRoxOktaAuthProviderRead")

	// Attempt to read from an upstream API.
	cli := meta.(ClientWrap)
	authProvider, resp, err := cli.AuthProviderServiceApi.GetAuthProvider(cli.BasicAuthContext(), data.Id())
	logResult(authProvider, resp, err)

	// If the resource does not exist, inform Terraform. We want to immediately
	// return here to prevent further processing.
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		data.SetId("")
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	groups, resp, err := cli.FindGroups(cli.BasicAuthContext(), data.Id())
	logResult(authProvider, resp, err)

	if err != nil {
		return nil, err
	}

	// Update the local state.
	if err := stackRoxOktaAuthProviderSetState(data, authProvider, groups); err != nil {
		return nil, fmt.Errorf("error importing resource: %v", err)
	}

	return []*schema.ResourceData{data}, nil
}
