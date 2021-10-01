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
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/stretchr/testify/assert"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

// TestAccStackRoxGenericImageRegistry_basic exercises the code in real plan, apply,
// refresh, and destroy life cycles for the `stackrox_generic_image_registry` resource.
func TestAccStackRoxGenericImageRegistry_basic(t *testing.T) {
	resourceName := acctest.RandomWithPrefix("testacc-generic-image-registry")

	var registry stackrox.StorageImageIntegration

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders(),
		Steps: []resource.TestStep{
			// Exercise the plan, apply, refresh, and destroy life cycles.
			{
				Config: testAccStackRoxGenericImageRegistryConfig(resourceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckStackRoxGenericImageRegistryExists(resourceName, &registry),
					testAccCheckStackRoxGenericImageRegistryResourceAttributes(testAccStackRoxGenericImageRegistryAddress(resourceName), &registry),
				),
			},
			// Exercise the import life cycle.
			{
				ResourceName:      testAccStackRoxGenericImageRegistryAddress(resourceName),
				Config:            testAccStackRoxProviderConfig(),
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"password",
				},
				ImportStateId: resourceName,
			},
		},
		CheckDestroy: testAccCheckStackRoxGenericImageRegistryWasDestroyed(resourceName),
	})
}

func TestAccStackRoxGenericImageRegistry_destroyIsIdempotent(t *testing.T) {
	t.Parallel()

	id := acctest.RandString(10)
	data := &schema.ResourceData{}
	data.SetId(id)

	err := resourceStackRoxGenericImageRegistry().Delete(data, testAccClientWrap())
	assert.NoError(t, err)
}

func testAccCheckStackRoxGenericImageRegistryResourceAttributes(resourceName string, registry *stackrox.StorageImageIntegration) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		// The API returns an empty string for password. So, it's overwritten here
		// in order to be verified below.
		registry.Docker.Password = "bob"

		resource.TestCheckResourceAttr(resourceName, "name", registry.Name)
		resource.TestCheckResourceAttr(resourceName, "endpoint", registry.Docker.Endpoint)
		resource.TestCheckResourceAttr(resourceName, "username", registry.Docker.Username)
		resource.TestCheckResourceAttr(resourceName, "password", registry.Docker.Password)
		return nil
	}
}

func testAccCheckStackRoxGenericImageRegistryExists(resourceName string, out *stackrox.StorageImageIntegration) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[testAccStackRoxGenericImageRegistryAddress(resourceName)]
		if !ok {
			return fmt.Errorf("not found: %s", testAccStackRoxGenericImageRegistryAddress(resourceName))
		}

		if res.Primary.ID == "" {
			return fmt.Errorf("no generic-image-registry ID is set")
		}

		cli := testAccClientWrap()

		result, resp, err := cli.ImageIntegrationServiceApi.GetImageIntegration(cli.BasicAuthContext(), res.Primary.ID)

		*out = result

		if err != nil {
			return fmt.Errorf("error fetching resource: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status is not OK: %s", resp.Status)
		}

		return nil
	}
}

func testAccStackRoxGenericImageRegistryConfig(resourceName string) string {
	const config = testAccProviderConfig + `
resource "stackrox_generic_image_registry" "%s" {
  name     = "%s"
  endpoint = "example.com"
  username = "alice"
  password = "bob"
}
`

	return fmt.Sprintf(config, testAccEndpoint(), testAccPassword(), resourceName, resourceName)
}

func testAccCheckStackRoxGenericImageRegistryWasDestroyed(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[testAccStackRoxGenericImageRegistryAddress(resourceName)]
		if !ok {
			return fmt.Errorf("not found: %s", testAccStackRoxGenericImageRegistryAddress(resourceName))
		}

		if res.Primary.ID == "" {
			return fmt.Errorf("no image-registry ID is set")
		}

		cli := testAccClientWrap()

		_, resp, _ := cli.ImageIntegrationServiceApi.GetImageIntegration(cli.BasicAuthContext(), res.Primary.ID)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("remote image-registry resource was not destroyed. status: %v", resp.Status)
		}

		return nil
	}
}

func testAccStackRoxGenericImageRegistryAddress(resourceName string) string {
	return fmt.Sprintf("stackrox_generic_image_registry.%s", resourceName)
}
