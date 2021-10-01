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
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"

	"github.com/splunk/terraform-provider-stackrox/internal/provider/stackrox"
)

func resourceStackRoxPolicy() *schema.Resource {
	return &schema.Resource{
		Create:   stackRoxPolicyCreate,
		Read:     stackRoxPolicyRead,
		Update:   stackRoxPolicyUpdate,
		Delete:   stackRoxPolicyDelete,
		Importer: stackRoxPolicyImporter(),
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Required: true,
			},
			"rationale": {
				Type:     schema.TypeString,
				Required: true,
			},
			"remediation": {
				Type:     schema.TypeString,
				Required: true,
			},
			"disabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"categories": {
				Type:     schema.TypeSet,
				Required: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"lifecycle_stages": {
				Type:     schema.TypeSet,
				Required: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"severity": {
				Type:     schema.TypeString,
				Required: true,
			},
			"notifiers": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"policy_criteria": {
				Type:     schema.TypeList,
				MaxItems: 1,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"cvss": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"privileged": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
		},
	}
}

var severityMap = map[string]stackrox.StorageSeverity{
	"LOW_SEVERITY":      stackrox.STORAGESEVERITY_LOW_SEVERITY,
	"MEDIUM_SEVERITY":   stackrox.STORAGESEVERITY_MEDIUM_SEVERITY,
	"HIGH_SEVERITY":     stackrox.STORAGESEVERITY_HIGH_SEVERITY,
	"CRITICAL_SEVERITY": stackrox.STORAGESEVERITY_CRITICAL_SEVERITY,
}

func severityFrom(s string) (stackrox.StorageSeverity, error) {
	result, ok := severityMap[s]
	if !ok {
		return "", fmt.Errorf("Unsupported severity")
	}
	return result, nil
}

var lifecycleStagesMap = map[string]stackrox.StorageLifecycleStage{
	"BUILD":   stackrox.STORAGELIFECYCLESTAGE_BUILD,
	"DEPLOY":  stackrox.STORAGELIFECYCLESTAGE_DEPLOY,
	"RUNTIME": stackrox.STORAGELIFECYCLESTAGE_RUNTIME,
}

func categoriesFrom(l []interface{}) ([]string, error) {
	result := make([]string, 0, len(l))

	for _, e := range l {
		s := e.(string)
		result = append(result, s)
	}

	return result, nil
}

func lifecycleStagesFrom(l []interface{}) ([]stackrox.StorageLifecycleStage, error) {
	result := make([]stackrox.StorageLifecycleStage, 0, len(l))

	for _, e := range l {
		s := e.(string)
		r, ok := lifecycleStagesMap[s]
		if !ok {
			return nil, fmt.Errorf("Unsupported lifecyle stage")
		}
		result = append(result, r)
	}

	return result, nil
}

func notifiersFrom(l []interface{}) []string {
	result := make([]string, 0, len(l))

	for _, e := range l {
		s := e.(string)
		result = append(result, s)
	}

	return result
}

func stackRoxPolicyCreate(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxPolicyCreate")

	message, err := stackRoxPolicyMessageFrom(data)
	if err != nil {
		return err
	}
	logMessage(message)

	cli := meta.(ClientWrap)
	result, resp, err := cli.PolicyServiceApi.PostPolicy(cli.BasicAuthContext(), message)
	logResult(result, resp, err)

	if err != nil {
		return err
	}

	// Set the ID of the resource to the cluster_id. A non-blank ID
	// tells Terraform that a resource was created.
	data.SetId(result.Id)
	return stackRoxPolicyRead(data, meta)
}

func stackRoxBoolPtrFromTerraform(data interface{}) *bool {
	if data == nil {
		return nil
	}

	value := data.(string)
	if value == "" {
		return nil
	}

	boolval, err := strconv.ParseBool(value)
	if err != nil {
		panic(err)
	}

	return &boolval
}

func stackRoxPolicyRead(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxPolicyRead")

	// Attempt to read from an upstream API.
	cli := meta.(ClientWrap)
	result, resp, err := cli.PolicyServiceApi.GetPolicy(cli.BasicAuthContext(), data.Id())
	logResult(result, resp, err)

	// If the resource does not exist, inform Terraform. We want to immediately
	// return here to prevent further processing.
	if resp.StatusCode == http.StatusNotFound {
		data.SetId("")
		return nil
	}

	if err != nil {
		return err
	}

	// Update the local state.
	return stackRoxPolicySetState(data, result)
}

func stackRoxPolicyUpdate(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxPolicyUpdate")

	if !data.HasChanges("name", "description", "rationale", "remediation", "disabled",
		"categories", "lifecycle_stages", "severity", "notifiers", "policy_criteria") {
		return stackRoxPolicyRead(data, meta)
	}

	message, err := stackRoxPolicyMessageFrom(data)
	if err != nil {
		return err
	}
	logMessage(message)

	cli := meta.(ClientWrap)
	result, resp, err := cli.PolicyServiceApi.PutPolicy(cli.BasicAuthContext(), data.Id(), message)
	logResult(result, resp, err)

	if err != nil {
		return err
	}

	return stackRoxPolicyRead(data, meta)
}

func stackRoxPolicyDelete(data *schema.ResourceData, meta interface{}) error {
	debug("calling stackRoxPolicyDelete: " + data.Id())

	// Attempt to delete from an upstream API.
	// data.SetId("") is automatically called assuming delete returns no errors.
	cli := meta.(ClientWrap)
	result, resp, err := cli.PolicyServiceApi.DeletePolicy(cli.BasicAuthContext(), data.Id())
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

func stackRoxPolicyImporter() *schema.ResourceImporter {
	return &schema.ResourceImporter{
		State: stackRoxPolicyImportState,
	}
}

// Import by name.
func stackRoxPolicyImportState(data *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	debug("calling stackRoxPolicyImportState")

	// Attempt to read from an upstream API, using the name as a natural key.
	cli := meta.(ClientWrap)
	result, resp, err := cli.PolicyServiceApi.GetPolicy(cli.BasicAuthContext(), data.Id())
	logResult(result, resp, err)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(resp.Status)
	}

	// Import the resource.
	if err := stackRoxPolicySetState(data, result); err != nil {
		return nil, fmt.Errorf("error importing resource: %v", err)
	}

	return []*schema.ResourceData{data}, nil
}

func stackRoxPolicySetState(data *schema.ResourceData, src stackrox.StoragePolicy) error {
	if err := data.Set("name", src.Name); err != nil {
		return err
	}
	if err := data.Set("description", src.Description); err != nil {
		return err
	}
	if err := data.Set("rationale", src.Rationale); err != nil {
		return err
	}
	if err := data.Set("remediation", src.Remediation); err != nil {
		return err
	}
	if err := data.Set("disabled", src.Disabled); err != nil {
		return err
	}
	if err := data.Set("categories", src.Categories); err != nil {
		return err
	}
	if err := data.Set("lifecycle_stages", src.LifecycleStages); err != nil {
		return err
	}
	if err := data.Set("severity", src.Severity); err != nil {
		return err
	}
	if err := data.Set("notifiers", src.Notifiers); err != nil {
		return err
	}

	// policy_criteria
	criteria := []map[string]interface{}{
		{
			"cvss":       stackRoxTerraformCvssFromMessage(src.Fields.Cvss),
			"privileged": stackRoxTerraformBoolFromMessage(src.Fields.Privileged),
		},
	}
	if err := data.Set("policy_criteria", criteria); err != nil {
		return err
	}

	return nil
}

func stackRoxPolicyMessageFrom(data *schema.ResourceData) (message stackrox.StoragePolicy, err error) {
	categories, err := categoriesFrom(data.Get("categories").(*schema.Set).List())
	if err != nil {
		return
	}

	lifecycleStages, err := lifecycleStagesFrom(data.Get("lifecycle_stages").(*schema.Set).List())
	if err != nil {
		return
	}

	severity, err := severityFrom(data.Get("severity").(string))
	if err != nil {
		return
	}

	notifiers := notifiersFrom(data.Get("notifiers").(*schema.Set).List())

	policyCriteria := data.Get("policy_criteria").([]interface{})
	criteria := policyCriteria[0].(map[string]interface{})
	log.Println(criteria)

	message = stackrox.StoragePolicy{
		Name:            data.Get("name").(string),
		Description:     data.Get("description").(string),
		Rationale:       data.Get("rationale").(string),
		Remediation:     data.Get("remediation").(string),
		Disabled:        data.Get("disabled").(bool),
		Categories:      categories,
		LifecycleStages: lifecycleStages,
		Severity:        severity,
		Notifiers:       notifiers,
		Fields: stackrox.StoragePolicyFields{
			Cvss:       stackRoxCvssFromTerraform(criteria["cvss"]),
			Privileged: stackRoxBoolPtrFromTerraform(criteria["privileged"]),
		},
	}

	return
}

var stackRoxCvssSymbolToOp = map[string]stackrox.StorageComparator{
	"=":  stackrox.STORAGECOMPARATOR_EQUALS,
	">":  stackrox.STORAGECOMPARATOR_GREATER_THAN,
	">=": stackrox.STORAGECOMPARATOR_GREATER_THAN_OR_EQUALS,
	"<":  stackrox.STORAGECOMPARATOR_LESS_THAN,
	"<=": stackrox.STORAGECOMPARATOR_LESS_THAN_OR_EQUALS,
}

var stackRoxCvssOpsToSymbols = map[stackrox.StorageComparator]string{
	stackrox.STORAGECOMPARATOR_EQUALS:                 "=",
	stackrox.STORAGECOMPARATOR_GREATER_THAN:           ">",
	stackrox.STORAGECOMPARATOR_GREATER_THAN_OR_EQUALS: ">=",
	stackrox.STORAGECOMPARATOR_LESS_THAN:              "<",
	stackrox.STORAGECOMPARATOR_LESS_THAN_OR_EQUALS:    "<=",
}

func stackRoxTerraformCvssFromMessage(cvss *stackrox.StorageNumericalPolicy) (result string) {
	if cvss == nil {
		return
	}

	op, err := stackRoxCvssOpFrom(cvss.Op)
	if err != nil {
		panic(err)
	}

	result = fmt.Sprintf("%s %d", op, int32(cvss.Value))

	return
}

func stackRoxCvssFromTerraform(data interface{}) *stackrox.StorageNumericalPolicy {
	if data == nil {
		return nil
	}

	value := data.(string)
	if value == "" {
		return nil
	}

	var op string
	var val int32
	log.Println(value)
	if n, err := fmt.Sscanf(value, "%s %d", &op, &val); n != 2 || err != nil {
		panic(err)
	}

	return &stackrox.StorageNumericalPolicy{
		Op:    stackRoxCvssSymbolToOp[op],
		Value: float32(val),
	}
}

func stackRoxCvssOpFrom(op stackrox.StorageComparator) (result string, err error) {
	result, ok := stackRoxCvssOpsToSymbols[op]
	if !ok {
		err = fmt.Errorf("Invalid CVSS comparator")
	}
	return
}

func stackRoxTerraformBoolFromMessage(p *bool) string {
	if p == nil {
		return ""
	}

	return strconv.FormatBool(*p)
}
