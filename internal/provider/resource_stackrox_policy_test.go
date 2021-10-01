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

// TestAccStackRoxPolicy_basic exercises the code in real plan, apply,
// refresh, and destroy life cycles for the `stackrox_policy` resource.
func TestAccStackRoxPolicy_basic(t *testing.T) {
	resourceName := acctest.RandomWithPrefix("testacc-policy")

	var policy stackrox.StoragePolicy

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders(),
		Steps: []resource.TestStep{
			// Exercise the plan, apply, refresh, and destroy life cycles.
			{
				Config: testAccStackRoxPolicyConfig(resourceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckStackRoxPolicyExists(resourceName, &policy),
					testAccCheckStackRoxPolicyResourceAttributes(testAccStackRoxPolicyAddress(resourceName), &policy),
					resource.TestCheckNoResourceAttr(testAccStackRoxPolicyAddress(resourceName), "notifiers"),
				),
			},
			// Exercise the import life cycle.
			{
				ResourceName:      testAccStackRoxPolicyAddress(resourceName),
				Config:            testAccStackRoxProviderConfig(),
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update notifiers.
			{
				Config: testAccStackRoxPolicyConfigNotifiers(resourceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckStackRoxPolicyExists(resourceName, &policy),
					testAccCheckStackRoxPolicyResourceAttributes(testAccStackRoxPolicyAddress(resourceName), &policy),
					resource.TestCheckResourceAttr(testAccStackRoxPolicyAddress(resourceName), "notifiers.#", "1"),
				),
			},
		},
		CheckDestroy: testAccCheckStackRoxPolicyWasDestroyed(resourceName),
	})
}

func TestAccStackRoxPolicy_destroyIsIdempotent(t *testing.T) {
	t.Parallel()

	id := acctest.RandString(10)
	data := &schema.ResourceData{}
	data.SetId(id)

	err := resourceStackRoxPolicy().Delete(data, testAccClientWrap())
	assert.NoError(t, err)
}

func testAccCheckStackRoxPolicyResourceAttributes(resourceName string, policy *stackrox.StoragePolicy) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resource.TestCheckResourceAttr(resourceName, "name", policy.Name)
		resource.TestCheckResourceAttr(resourceName, "description", policy.Description)
		resource.TestCheckResourceAttr(resourceName, "rationale", policy.Rationale)
		resource.TestCheckResourceAttr(resourceName, "remediation", policy.Remediation)
		resource.TestCheckResourceAttr(resourceName, "disabled", strconv.FormatBool(policy.Disabled))
		resource.TestCheckResourceAttr(resourceName, "severity", string(policy.Severity))

		// verify categories
		resource.TestCheckResourceAttr(resourceName, "categories.#", strconv.FormatInt(int64(len(policy.Categories)), 10))
		for i, v := range policy.Categories {
			resource.TestCheckResourceAttr(resourceName, fmt.Sprintf("categories.%d", i), v)
		}

		// verify lifecycle stages
		resource.TestCheckResourceAttr(resourceName, "lifecycle_stages.#", strconv.FormatInt(int64(len(policy.LifecycleStages)), 10))
		for i, v := range policy.LifecycleStages {
			resource.TestCheckResourceAttr(resourceName, fmt.Sprintf("lifecycle_stages.%d", i), string(v))
		}

		// verify policy criteria
		resource.TestCheckResourceAttr(resourceName, "policy_criteria.#", "1")
		resource.TestCheckResourceAttr(resourceName, "policy_criteria.0.cvss", stackRoxTerraformCvssFromMessage(policy.Fields.Cvss))
		resource.TestCheckResourceAttr(resourceName, "policy_criteria.0.privileged", stackRoxTerraformBoolFromMessage(policy.Fields.Privileged))

		return nil
	}
}

func testAccCheckStackRoxPolicyExists(resourceName string, out *stackrox.StoragePolicy) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[testAccStackRoxPolicyAddress(resourceName)]
		if !ok {
			return fmt.Errorf("not found: %s", testAccStackRoxPolicyAddress(resourceName))
		}

		if res.Primary.ID == "" {
			return fmt.Errorf("no policy ID is set")
		}

		cli := testAccClientWrap()

		result, resp, err := cli.PolicyServiceApi.GetPolicy(cli.BasicAuthContext(), res.Primary.ID)

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

func testAccStackRoxPolicyConfig(resourceName string) string {
	const config = testAccProviderConfig + `
resource "stackrox_splunk_integration" "%s" {
  name                  = "%s"
  hec_endpoint          = "http://example.com"
  hec_token             = "testing"
  truncate              = 10000
  ui_endpoint           = "http://localhost"
  audit_logging_enabled = false
}

resource "stackrox_policy" "%s" {
  name             = "%s"
  description      = "fake description"
  rationale        = "fake rationale"
  remediation      = "fake remediation"
  disabled         = true
  categories       = [
    "DevOps Best Practices",
    "Security Best Practices"
  ]
  lifecycle_stages = [
    "DEPLOY"
  ]
  severity         = "HIGH_SEVERITY"

  policy_criteria {
    cvss = ">= 3"
    privileged = true
  }
}
`
	return fmt.Sprintf(config, testAccEndpoint(), testAccPassword(),
		resourceName, resourceName, resourceName, resourceName,
	)
}

func testAccStackRoxPolicyConfigNotifiers(resourceName string) string {
	const config = testAccProviderConfig + `
resource "stackrox_splunk_integration" "%s" {
  name                  = "%s"
  hec_endpoint          = "http://example.com"
  hec_token             = "testing"
  truncate              = 10000
  ui_endpoint           = "http://localhost"
  audit_logging_enabled = false
}

resource "stackrox_policy" "%s" {
  name             = "%s"
  description      = "fake description"
  rationale        = "fake rationale"
  remediation      = "fake remediation"
  disabled         = true
  categories       = [
    "DevOps Best Practices",
    "Security Best Practices"
  ]
  lifecycle_stages = [
    "DEPLOY"
  ]
  severity         = "HIGH_SEVERITY"
  notifiers        = [
    stackrox_splunk_integration.%s.id
  ]

  policy_criteria {
    cvss = ">= 3"
    privileged = true
  }
}
`
	return fmt.Sprintf(config, testAccEndpoint(), testAccPassword(),
		resourceName, resourceName, resourceName, resourceName, resourceName,
	)
}

func testAccCheckStackRoxPolicyWasDestroyed(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[testAccStackRoxPolicyAddress(resourceName)]
		if !ok {
			return fmt.Errorf("not found: %s", testAccStackRoxPolicyAddress(resourceName))
		}

		if res.Primary.ID == "" {
			return fmt.Errorf("no policy ID is set")
		}

		cli := testAccClientWrap()

		_, resp, _ := cli.PolicyServiceApi.GetPolicy(cli.BasicAuthContext(), res.Primary.ID)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("remote policy resource was not destroyed. status: %v", resp.Status)
		}

		return nil
	}
}

func testAccStackRoxPolicyAddress(resourceName string) string {
	return fmt.Sprintf("stackrox_policy.%s", resourceName)
}
