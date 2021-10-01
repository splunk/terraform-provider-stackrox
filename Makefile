#   Copyright 2021 Splunk Inc.
#
#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at
#
#       http://www.apache.org/licenses/LICENSE-2.0
#
#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.

SED := $(shell which sed)

SWAGGER_PATCH_JQ_FILTER := '.paths["/v1/networkpolicies/cluster/{clusterId}"].get.operationId = "GetNetworkPolicyGraph" | .paths["/v1/rbac/roles/{id}"].get.operationId = "GetRBACRole"'
SWAGGER_PATCHED_FILE := swagger-patched.json
GENERATED_STACKROX_API_CLIENT_PACKAGE := internal/provider/stackrox

ifdef CI_JOB_ID
VOLUME_FLAGS := --volumes-from `docker ps -q`
else
VOLUME_FLAGS := -v $(CURDIR):/builds/kub/stackrox
endif

.PHONY: all
all: provider test

.PHONY: provider
provider: $(GENERATED_STACKROX_API_CLIENT_PACKAGE)
	@echo "+ $@"
	CGO_ENABLED=0 go build -a -ldflags '-extldflags "-static"' -o terraform-provider-stackrox .

.PHONY: test
test:
	@echo "+ $@"
	go test ./... -v -timeout 2m

.PHONY: testacc
testacc:
	@echo "+ $@"
	TF_ACC=1 go test ./internal/provider -v -timeout 10m

$(GENERATED_STACKROX_API_CLIENT_PACKAGE): swagger.json
	@echo "+ $@"
	$(MAKE) generate

# Patch the downloaded OpenAPI specification and patch it because it has bugs. Then, generate the Golang client.
# Remove the extraneous go.mod and go.sum files because they interfere with the build.
.PHONY: swagger
swagger:
	@echo "+ $@"
	jq $(SWAGGER_PATCH_JQ_FILTER) swagger.json >$(SWAGGER_PATCHED_FILE)
	$(RM) -r $(GENERATED_STACKROX_API_CLIENT_PACKAGE)
	docker run --rm $(VOLUME_FLAGS) openapitools/openapi-generator-cli:v4.2.0 generate \
		-i /builds/kub/stackrox/$(SWAGGER_PATCHED_FILE) \
		-g go \
		-o /builds/kub/stackrox/$(GENERATED_STACKROX_API_CLIENT_PACKAGE) \
		--additional-properties packageName=stackrox,withGoCodegenComment=true,enumClassPrefix=true
	$(RM) $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/go.mod
	$(RM) $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/go.sum
	mv $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_security_context_se_linux.go $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_security_context_selinux.go

.PHONY: patch
patch:
	@echo "+ $@"
	$(SED) -f model_storage_cluster.sed $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_cluster.go > $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_cluster.go.new
	mv $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_cluster.go.new $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_cluster.go
	$(SED) -f model_storage_image_integration.sed $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_image_integration.go > $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_image_integration.go.new
	mv $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_image_integration.go.new $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_image_integration.go
	$(SED) -f model_storage_notifier.sed $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_notifier.go > $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_notifier.go.new
	mv $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_notifier.go.new $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_notifier.go
	$(SED) -f model_storage_policy_fields.sed $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_policy_fields.go > $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_policy_fields.go.new
	mv $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_policy_fields.go.new $(GENERATED_STACKROX_API_CLIENT_PACKAGE)/model_storage_policy_fields.go

.PHONY: generate
generate: swagger patch
