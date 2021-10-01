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
	"log"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/stretchr/testify/assert"
)

func TestProvider(t *testing.T) {
	t.Parallel()
	err := Provider().(*schema.Provider).InternalValidate()
	assert.NoError(t, err)
}

func TestProvider_impl(t *testing.T) {
	t.Parallel()
	var _ = Provider()
}

func testAccProviders() map[string]terraform.ResourceProvider {
	return map[string]terraform.ResourceProvider{
		"stackrox": Provider(),
	}
}

func testAccClientWrap() ClientWrap {
	return NewClientWrap(testAccEndpoint(), "admin", testAccPassword())
}

func testAccStackRoxProviderConfig() string {
	return fmt.Sprintf(testAccProviderConfig, testAccEndpoint(), testAccPassword())
}

func testAccEndpoint() string {
	return lookupEnvOrFail("stackrox_api_endpoint")
}

func testAccPassword() string {
	return lookupEnvOrFail("stackrox_api_admin_password")
}

func lookupEnvOrFail(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		log.Fatalf("missing environment variable: %s", key)
	}
	return val
}

const testAccProviderConfig = `
provider "stackrox" {
  endpoint       = "%s"
  admin_password = "%s"
}
`
