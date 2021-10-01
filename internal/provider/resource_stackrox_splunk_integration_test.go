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
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/stretchr/testify/assert"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

// TestAccStackRoxSplunkIntegration_basic exercises the code in real plan, apply,
// refresh, and destroy life cycles for the `stackrox_splunk_integration` resource.
func TestAccStackRoxSplunkIntegration_basic(t *testing.T) {
	resourceName := acctest.RandomWithPrefix("testacc-splunk-integration")

	var notifier stackrox.StorageNotifier

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders(),
		Steps: []resource.TestStep{
			// Exercise the plan, apply, refresh, and destroy life cycles.
			{
				Config: testAccStackRoxSplunkIntegrationConfig(resourceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckStackRoxSplunkIntegrationExists(resourceName, &notifier),
					testAccCheckStackRoxSplunkIntegrationResourceAttributes(testAccStackRoxSplunkIntegrationAddress(resourceName), &notifier),
				),
			},
			// Exercise the import life cycle.
			{
				ResourceName:      testAccStackRoxSplunkIntegrationAddress(resourceName),
				Config:            testAccStackRoxProviderConfig(),
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"hec_token",
				},
				ImportStateId: notifier.Id,
			},
		},
		CheckDestroy: testAccCheckStackRoxSplunkIntegrationWasDestroyed(resourceName),
	})
}

func TestAccStackRoxSplunkIntegration_destroyIsIdempotent(t *testing.T) {
	t.Parallel()

	id := acctest.RandString(10)
	data := &schema.ResourceData{}
	data.SetId(id)

	err := resourceStackRoxSplunkIntegration().Delete(data, testAccClientWrap())
	assert.NoError(t, err)
}

func testAccCheckStackRoxSplunkIntegrationResourceAttributes(resourceName string, notifier *stackrox.StorageNotifier) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		// The API returns an empty string for password. So, it's overwritten here
		// in order to be verified below.
		notifier.Splunk.HttpToken = "testing"

		resource.TestCheckResourceAttr(resourceName, "name", notifier.Name)
		resource.TestCheckResourceAttr(resourceName, "hec_endpoint", notifier.Splunk.HttpEndpoint)
		resource.TestCheckResourceAttr(resourceName, "hec_token", notifier.Splunk.HttpToken)
		resource.TestCheckResourceAttr(resourceName, "truncate", notifier.Splunk.Truncate)
		resource.TestCheckResourceAttr(resourceName, "ui_endpoint", notifier.UiEndpoint)
		resource.TestCheckResourceAttr(resourceName, "audit_logging_enabled", strconv.FormatBool(notifier.Splunk.AuditLoggingEnabled))
		return nil
	}
}

func testAccCheckStackRoxSplunkIntegrationExists(resourceName string, out *stackrox.StorageNotifier) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[testAccStackRoxSplunkIntegrationAddress(resourceName)]
		if !ok {
			return fmt.Errorf("not found: %s", testAccStackRoxSplunkIntegrationAddress(resourceName))
		}

		if res.Primary.ID == "" {
			return fmt.Errorf("no splunk-integration ID is set")
		}

		cli := testAccClientWrap()

		result, resp, err := cli.NotifierServiceApi.GetNotifier(cli.BasicAuthContext(), res.Primary.ID)

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

func testAccStackRoxSplunkIntegrationConfig(resourceName string) string {
	const config = testAccProviderConfig + `
resource "stackrox_splunk_integration" "%s" {
  name                  = "%s"
  hec_endpoint          = "http://example.com"
  hec_token             = "testing"
  truncate              = 10000
  ui_endpoint           = "http://localhost"
  audit_logging_enabled = true
}
`

	return fmt.Sprintf(config, testAccEndpoint(), testAccPassword(), resourceName, resourceName)
}

func testAccCheckStackRoxSplunkIntegrationWasDestroyed(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[testAccStackRoxSplunkIntegrationAddress(resourceName)]
		if !ok {
			return fmt.Errorf("not found: %s", testAccStackRoxSplunkIntegrationAddress(resourceName))
		}

		if res.Primary.ID == "" {
			return fmt.Errorf("no splunk-integration ID is set")
		}

		cli := testAccClientWrap()

		_, resp, _ := cli.ImageIntegrationServiceApi.GetImageIntegration(cli.BasicAuthContext(), res.Primary.ID)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("remote splunk-integration resource was not destroyed. status: %v", resp.Status)
		}

		return nil
	}
}

func testAccStackRoxSplunkIntegrationAddress(resourceName string) string {
	return fmt.Sprintf("stackrox_splunk_integration.%s", resourceName)
}
