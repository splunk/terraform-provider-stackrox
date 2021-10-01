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
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

func resourceStackRoxKubernetesCluster() *schema.Resource {
	return &schema.Resource{
		Create:   stackRoxKubernetesClusterCreate,
		Read:     stackRoxKubernetesClusterRead,
		Update:   stackRoxKubernetesClusterUpdate,
		Delete:   stackRoxKubernetesClusterDelete,
		Importer: stackRoxKubernetesClusterImporter(),
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"cluster_id": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringMatch(regexp.MustCompile("([a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}){1}"), "not a UUID"),
			},
			"central_api_endpoint": {
				Type:     schema.TypeString,
				Required: true,
			},
			"collection_method": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					string(stackrox.STORAGECOLLECTIONMETHOD_NO_COLLECTION),
					string(stackrox.STORAGECOLLECTIONMETHOD_KERNEL_MODULE),
					string(stackrox.STORAGECOLLECTIONMETHOD_EBPF),
				}, false),
			},
			"runtime_support": {
				Type:     schema.TypeBool,
				Required: true,
			},
		},
	}
}

func stackRoxKubernetesClusterCreate(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxKubernetesClusterCreate")

	clusterID := data.Get("cluster_id").(string)

	message := stackrox.StorageCluster{
		CentralApiEndpoint: data.Get("central_api_endpoint").(string),
		CollectionMethod:   stackrox.StorageCollectionMethod(data.Get("collection_method").(string)),
		CollectorImage:     "cloudrepo-docker.jfrog.io/kub/collector.stackrox.io/collector",
		Id:                 clusterID,
		MainImage:          "cloudrepo-docker.jfrog.io/kub/stackrox.io/main",
		Name:               data.Get("name").(string),
		RuntimeSupport:     data.Get("runtime_support").(bool),
		Type:               "KUBERNETES_CLUSTER",
	}

	logMessage(message)

	cli := meta.(ClientWrap)
	result, resp, err := cli.ClustersServiceApi.PutCluster(cli.BasicAuthContext(), clusterID, message)
	logResult(result, resp, err)
	if err != nil {
		return err
	}

	// Set the ID of the resource to the cluster_id. A non-blank ID
	// tells Terraform that a resource was created.
	data.SetId(clusterID)
	return stackRoxKubernetesClusterRead(data, meta)
}

func stackRoxKubernetesClusterRead(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxKubernetesClusterRead")

	// Attempt to read from an upstream API.
	cli := meta.(ClientWrap)
	result, resp, err := cli.ClustersServiceApi.GetCluster(cli.BasicAuthContext(), data.Get("cluster_id").(string))
	logResult(result, resp, err)

	// If the resource does not exist, inform Terraform. We want to immediately
	// return here to prevent further processing.
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		data.SetId("")
		return nil
	}

	if err != nil {
		return err
	}

	// Update the local state.
	if err := stackRoxKubernetesClusterSetStateData(data, result); err != nil {
		return fmt.Errorf("error saving resource: %v", err)
	}

	return nil
}

func stackRoxKubernetesClusterUpdate(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxKubernetesClusterUpdate")

	if !data.HasChanges("name", "central_api_endpoint", "collection_method", "runtime_support") {
		return stackRoxKubernetesClusterRead(data, meta)
	}

	clusterID := data.Get("cluster_id").(string)

	message := stackrox.StorageCluster{
		CentralApiEndpoint: data.Get("central_api_endpoint").(string),
		CollectionMethod:   stackrox.StorageCollectionMethod(data.Get("collection_method").(string)),
		CollectorImage:     "cloudrepo-docker.jfrog.io/kub/collector.stackrox.io/collector",
		Id:                 clusterID,
		MainImage:          "cloudrepo-docker.jfrog.io/kub/stackrox.io/main",
		Name:               data.Get("name").(string),
		RuntimeSupport:     data.Get("runtime_support").(bool),
		Type:               "KUBERNETES_CLUSTER",
	}

	logMessage(message)

	cli := meta.(ClientWrap)
	result, resp, err := cli.ClustersServiceApi.PutCluster(cli.BasicAuthContext(), clusterID, message)
	logResult(result, resp, err)
	if err != nil {
		return err
	}

	return stackRoxKubernetesClusterRead(data, meta)
}

func stackRoxKubernetesClusterDelete(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxKubernetesClusterDelete: " + data.Id())

	// Attempt to delete from an upstream API.
	// data.SetId("") is automatically called assuming delete returns no errors.
	cli := meta.(ClientWrap)
	result, resp, err := cli.ClustersServiceApi.DeleteCluster(cli.BasicAuthContext(), data.Id())
	logResult(result, resp, err)

	// Destroy should be idempotent. The cluster API returns 500 when the resource isn't found.
	if resp != nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusInternalServerError) {
		return nil
	}

	if err != nil {
		return err
	}

	return fmt.Errorf(resp.Status)
}

func stackRoxKubernetesClusterImporter() *schema.ResourceImporter {
	return &schema.ResourceImporter{
		State: stackRoxKubernetesClusterImportState,
	}
}

func stackRoxKubernetesClusterImportState(data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	debug("calling stackRoxKubernetesClusterImportState")

	// Attempt to read from an upstream API.
	cli := meta.(ClientWrap)
	result, resp, err := cli.ClustersServiceApi.GetCluster(cli.BasicAuthContext(), data.Id())
	logResult(result, resp, err)
	if err != nil {
		return nil, err
	}

	// Import the resource.
	if err := stackRoxKubernetesClusterSetStateData(data, result); err != nil {
		return nil, fmt.Errorf("error importing resource: %v", err)
	}

	return []*schema.ResourceData{data}, nil
}

func stackRoxKubernetesClusterSetStateData(data *schema.ResourceData, src stackrox.V1ClusterResponse) error {
	if err := data.Set("central_api_endpoint", src.Cluster.CentralApiEndpoint); err != nil {
		return err
	}
	if err := data.Set("cluster_id", src.Cluster.Id); err != nil {
		return err
	}
	if err := data.Set("collection_method", src.Cluster.CollectionMethod); err != nil {
		return err
	}
	if err := data.Set("name", src.Cluster.Name); err != nil {
		return err
	}
	if err := data.Set("runtime_support", src.Cluster.RuntimeSupport); err != nil {
		return err
	}
	return nil
}
