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
// refresh, and destroy life cycles for the `stackrox_okta_auth_provider` resource.
func TestAccStackRoxOktaAuthProvider_basic(t *testing.T) {
	t.Skip("Skipped because StackRox won't accept a test config with a fake IDP metadata URL.")
	resourceName := acctest.RandomWithPrefix("testacc-okta-auth-provider")

	var authProvider stackrox.StorageAuthProvider
	var groups []stackrox.StorageGroup

	resource.Test(t, resource.TestCase{
		Providers: testAccProviders(),
		Steps: []resource.TestStep{
			// Exercise the plan, apply, refresh, and destroy life cycles.
			{
				Config: testAccStackRoxOktaAuthProviderConfig(resourceName),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckStackRoxOktaAuthProviderExists(resourceName, &authProvider, &groups),
					testAccCheckStackRoxOktaAuthProviderResourceAttributes(testAccStackRoxOktaAuthProviderAddress(resourceName), authProvider, groups),
				),
			},
			// Exercise the import life cycle.
			{
				ResourceName:      testAccStackRoxOktaAuthProviderAddress(resourceName),
				Config:            testAccStackRoxProviderConfig(),
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
		CheckDestroy: testAccCheckStackRoxOktaAuthProviderWasDestroyed(resourceName),
	})
}

func TestAccStackRoxOktaAuthProvider_destroyIsIdempotent(t *testing.T) {
	t.Parallel()

	id := acctest.RandString(10)
	data := &schema.ResourceData{}
	data.SetId(id)

	err := resourceStackRoxOktaAuthProvider().Delete(data, testAccClientWrap())
	assert.NoError(t, err)
}

func testAccStackRoxOktaAuthProviderConfig(resourceName string) string {
	const config = testAccProviderConfig + `
resource "stackrox_okta_auth_provider" "%s" {
  name             = "%s"
  type             = "saml"
  ui_endpoint      = "https://localhost/"
  enabled          = true
  idp_metadata_url = "https://example.com/"
  sp_issuer        = "https://example.com/"

  group {
    role = "None"
  }

  group {
    key   = "groups"
    value = "blargle-flargle"
    role  = "Blargle Flargle"
  }
}
`
	return fmt.Sprintf(config,
		testAccEndpoint(),
		testAccPassword(),
		resourceName,
		resourceName,
	)
}

func testAccCheckStackRoxOktaAuthProviderExists(resourceName string, outAuthProvider *stackrox.StorageAuthProvider, outGroups *[]stackrox.StorageGroup) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[testAccStackRoxOktaAuthProviderAddress(resourceName)]
		if !ok {
			return fmt.Errorf("not found: %s", testAccStackRoxOktaAuthProviderAddress(resourceName))
		}

		if res.Primary.ID == "" {
			return fmt.Errorf("no auth provider ID is set")
		}

		cli := testAccClientWrap()

		result, resp, err := cli.AuthProviderServiceApi.GetAuthProvider(cli.BasicAuthContext(), res.Primary.ID)

		*outAuthProvider = result

		if err != nil {
			return fmt.Errorf("error fetching resource: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status is not OK: %s", resp.Status)
		}

		groups, resp, err := cli.FindGroups(cli.BasicAuthContext(), res.Primary.ID)

		if err != nil {
			return fmt.Errorf("error fetching groups: %v", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status is not OK: %s", resp.Status)
		}

		outGroups = &groups

		return nil
	}
}

func testAccStackRoxOktaAuthProviderAddress(resourceName string) string {
	return fmt.Sprintf("stackrox_okta_auth_provider.%s", resourceName)
}

func testAccCheckStackRoxOktaAuthProviderResourceAttributes(resourceName string, authProvider stackrox.StorageAuthProvider, groups []stackrox.StorageGroup) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resource.TestCheckResourceAttr(resourceName, "name", authProvider.Name)
		resource.TestCheckResourceAttr(resourceName, "type", authProvider.Type)
		resource.TestCheckResourceAttr(resourceName, "ui_endpoint", authProvider.UiEndpoint)
		resource.TestCheckResourceAttr(resourceName, "enabled", strconv.FormatBool(authProvider.Enabled))
		resource.TestCheckResourceAttr(resourceName, "idp_metadata_url", authProvider.Config["idp_metadata_url"])
		resource.TestCheckResourceAttr(resourceName, "sp_issuer", authProvider.Config["sp_issuer"])

		// verify auth provider groups
		resource.TestCheckResourceAttr(resourceName, "group.#", strconv.FormatInt(int64(len(groups)), 10))
		for i, g := range groups {
			resource.TestCheckResourceAttr(resourceName, fmt.Sprintf("group.%d.key", i), g.Props.Key)
			resource.TestCheckResourceAttr(resourceName, fmt.Sprintf("group.%d.value", i), g.Props.Value)
			resource.TestCheckResourceAttr(resourceName, fmt.Sprintf("group.%d.role", i), g.RoleName)
		}

		return nil
	}
}

func testAccCheckStackRoxOktaAuthProviderWasDestroyed(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[testAccStackRoxOktaAuthProviderAddress(resourceName)]
		if !ok {
			return fmt.Errorf("not found: %s", testAccStackRoxOktaAuthProviderAddress(resourceName))
		}

		if res.Primary.ID == "" {
			return fmt.Errorf("no Okta auth provider ID is set")
		}

		cli := testAccClientWrap()

		_, resp, _ := cli.AuthProviderServiceApi.GetAuthProvider(cli.BasicAuthContext(), res.Primary.ID)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("remote okta auth provider resource was not destroyed. status: %v", resp.Status)
		}

		return nil
	}
}
