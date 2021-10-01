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

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
	"github.com/stretchr/testify/assert"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

// TestAccStackRoxKubernetesCluster_basic exercises the code in real plan, apply,
// refresh, and destroy life cycles for the `stackrox_kubernetes_cluster` resource.
func TestAccStackRoxKubernetesCluster_basic(t *testing.T) {
	resourceName := acctest.RandomWithPrefix("testacc-kubernetes-cluster")
	clusterID := uuid.New()

	var cluster stackrox.V1ClusterResponse

	resource.ParallelTest(t, resource.TestCase{
		Providers: testAccProviders(),
		Steps: []resource.TestStep{
			// Exercise the plan, apply, refresh, and destroy life cycles.
			{
				Config: testAccStackRoxClusterConfig(resourceName, clusterID.String()),
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckStackRoxClusterExists(resourceName, &cluster),
					testAccCheckStackRoxClusterResourceAttributes(testAccStackRoxClusterResourceAddress(resourceName), &cluster),
					testAccCheckStackRoxClustersShouldNotAutoupgrade(),
				),
			},
			// Exercise the import life cycle.
			{
				ResourceName:      testAccStackRoxClusterResourceAddress(resourceName),
				Config:            testAccStackRoxProviderConfig(),
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
		CheckDestroy: testAccCheckStackRoxClusterWasDestroyed(resourceName),
	})
}

func TestAccStackRoxKubernetesCluster_destroyIsIdempotent(t *testing.T) {
	t.Parallel()

	id := acctest.RandString(10)
	data := &schema.ResourceData{}
	data.SetId(id)

	err := resourceStackRoxKubernetesCluster().Delete(data, testAccClientWrap())
	assert.NoError(t, err)
}

func testAccCheckStackRoxClusterResourceAttributes(resourceName string, cluster *stackrox.V1ClusterResponse) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		resource.TestCheckResourceAttr(resourceName, "cluster_id", cluster.Cluster.Id)
		resource.TestCheckResourceAttr(resourceName, "resourceName", cluster.Cluster.Name)
		resource.TestCheckResourceAttr(resourceName, "central_api_endpoint", cluster.Cluster.CentralApiEndpoint)
		resource.TestCheckResourceAttr(resourceName, "collection_method", string(cluster.Cluster.CollectionMethod))
		resource.TestCheckResourceAttr(resourceName, "runtime_support", strconv.FormatBool(cluster.Cluster.RuntimeSupport))
		return nil
	}
}

func testAccCheckStackRoxClustersShouldNotAutoupgrade() resource.TestCheckFunc {
	return func(state *terraform.State) error {
		cli := testAccClientWrap()

		result, _, err := cli.SensorUpgradeServiceApi.GetSensorUpgradeConfig(cli.BasicAuthContext())
		if err != nil {
			return err
		}

		if result.Config.EnableAutoUpgrade {
			return fmt.Errorf("Expected automatically upgrading secured clusters to be disabled!!!")
		}

		return nil
	}
}

func testAccStackRoxClusterConfig(resourceName, clusterID string) string {
	const config = testAccProviderConfig + `
resource "stackrox_kubernetes_cluster" "%s" {
  name                 = "%s"
  cluster_id           = "%s"
  central_api_endpoint = "central.stackrox:443"
  collection_method    = "KERNEL_MODULE"
  runtime_support      = true
}
`

	return fmt.Sprintf(config, testAccEndpoint(), testAccPassword(), resourceName, resourceName, clusterID)
}

func testAccStackRoxClusterResourceAddress(resourceName string) string {
	return fmt.Sprintf("stackrox_kubernetes_cluster.%s", resourceName)
}

func testAccCheckStackRoxClusterWasDestroyed(resourceName string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[testAccStackRoxClusterResourceAddress(resourceName)]
		if !ok {
			return fmt.Errorf("not found: %s", testAccStackRoxClusterResourceAddress(resourceName))
		}

		if res.Primary.ID == "" {
			return fmt.Errorf("no cluster ID is set")
		}

		cli := testAccClientWrap()

		_, resp, _ := cli.ClustersServiceApi.GetCluster(cli.BasicAuthContext(), res.Primary.ID)

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("remote cluster resource was not destroyed. status: %v", resp.Status)
		}

		return nil
	}
}

func testAccCheckStackRoxClusterExists(resourceName string, out *stackrox.V1ClusterResponse) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		res, ok := state.RootModule().Resources[testAccStackRoxClusterResourceAddress(resourceName)]
		if !ok {
			return fmt.Errorf("not found: %s", testAccStackRoxClusterResourceAddress(resourceName))
		}

		if res.Primary.ID == "" {
			return fmt.Errorf("no cluster ID is set")
		}

		cli := testAccClientWrap()

		result, resp, err := cli.ClustersServiceApi.GetCluster(cli.BasicAuthContext(), res.Primary.ID)

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
