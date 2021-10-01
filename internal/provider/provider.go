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
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"endpoint": {
				Type:     schema.TypeString,
				Required: true,
			},
			"admin_password": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"stackrox_generic_image_registry": resourceStackRoxGenericImageRegistry(),
			"stackrox_kubernetes_cluster":     resourceStackRoxKubernetesCluster(),
			"stackrox_okta_auth_provider":     resourceStackRoxOktaAuthProvider(),
			"stackrox_policy":                 resourceStackRoxPolicy(),
			"stackrox_splunk_integration":     resourceStackRoxSplunkIntegration(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(data *schema.ResourceData) (interface{}, error) {
	endpoint := data.Get("endpoint").(string)
	username := "admin"
	password := data.Get("admin_password").(string)

	client := NewClientWrap(endpoint, username, password)

	// Always disable automatic sensor upgrades.
	_, _, err := client.SensorUpgradeServiceApi.UpdateSensorUpgradeConfig(client.BasicAuthContext(), stackrox.V1UpdateSensorUpgradeConfigRequest{
		Config: stackrox.StorageSensorUpgradeConfig{
			EnableAutoUpgrade: false,
		},
	})

	return client, err
}
