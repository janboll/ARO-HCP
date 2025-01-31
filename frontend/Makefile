SHELL = /bin/bash
COMMIT = $(shell git rev-parse --short=7 HEAD)$(shell [[ $$(git status --porcelain) = "" ]] || echo -dirty)
ARO_HCP_BASE_IMAGE ?= ${ARO_HCP_IMAGE_ACR}.azurecr.io
ARO_HCP_FRONTEND_IMAGE ?= $(ARO_HCP_BASE_IMAGE)/arohcpfrontend:$(COMMIT)

# for deploying frontend into private aks cluster via invoke command
# these values must be set
RESOURCE_GROUP ?=
CLUSTER_NAME ?=
DEPLOYMENTNAME=$(RESOURCE_GROUP)


frontend:
	go build -o aro-hcp-frontend .

clean:
	rm -f aro-hcp-frontend

build-push: image push	

image:
	pushd .. && git archive --output frontend/archive.tar.gz HEAD && popd
	docker build --platform="linux/amd64" -f "./Dockerfile" -t ${ARO_HCP_FRONTEND_IMAGE} .
	rm -f archive.tar.gz

push:
	docker push ${ARO_HCP_FRONTEND_IMAGE}	

deploy:
	@test "${RESOURCE_GROUP}" != "" || (echo "RESOURCE_GROUP must be defined" && exit 1)
	FRONTEND_MI_CLIENT_ID=$(shell az deployment group show \
			-g ${RESOURCE_GROUP} \
			-n ${DEPLOYMENTNAME} \
			--query properties.outputs.frontend_mi_client_id.value);\
	DB_NAME=$(shell az cosmosdb list -g ${RESOURCE_GROUP} | jq -r '.[].name') DB_NAME=$${DB_NAME:-"none"};\
	oc process -f ./deploy/aro-hcp-frontend.yml --local \
		-p ARO_HCP_FRONTEND_IMAGE=${ARO_HCP_FRONTEND_IMAGE} \
		-p FRONTEND_MI_CLIENT_ID="$${FRONTEND_MI_CLIENT_ID}" \
		-p DB_NAME="$${DB_NAME}"| oc apply -f -

undeploy:
	@test "${RESOURCE_GROUP}" != "" || (echo "RESOURCE_GROUP must be defined" && exit 1)
	oc process -f ./deploy/aro-hcp-frontend.yml --local \
		-p ARO_HCP_FRONTEND_IMAGE=${ARO_HCP_FRONTEND_IMAGE} \
		-p FRONTEND_MI_CLIENT_ID="null" | oc delete -f -

deploy-private:
	@test "${RESOURCE_GROUP}" != "" && test "${CLUSTER_NAME}" != "" || (echo "RESOURCE_GROUP and CLUSTER_NAME must be defined" && exit 1)
	TMP_DEPLOY=$(shell mktemp);\
	FRONTEND_MI_CLIENT_ID=$(shell az deployment group show \
			-g ${RESOURCE_GROUP} \
			-n ${DEPLOYMENTNAME} \
			--query properties.outputs.frontend_mi_client_id.value);\
	DB_NAME=$(shell az cosmosdb list -g ${RESOURCE_GROUP} | jq -r '.[].name' || echo "none");\			
	oc process -f ./deploy/aro-hcp-frontend.yml --local \
		-p ARO_HCP_FRONTEND_IMAGE=${ARO_HCP_FRONTEND_IMAGE} \
		-p FRONTEND_MI_CLIENT_ID="$${FRONTEND_MI_CLIENT_ID}" \
		-p DB_NAME="$${DB_NAME}" > "$${TMP_DEPLOY}";\
	az aks command invoke --resource-group ${RESOURCE_GROUP} --name ${CLUSTER_NAME} --command "kubectl create -f $$(basename $${TMP_DEPLOY})" --file "$${TMP_DEPLOY}"

undeploy-private:
	@test "${RESOURCE_GROUP}" != "" && test "${CLUSTER_NAME}" != "" || (echo "RESOURCE_GROUP and CLUSTER_NAME must be defined" && exit 1)
	TMP_DEPLOY=$(shell mktemp);\
	oc process -f ./deploy/aro-hcp-frontend.yml --local \
		-p ARO_HCP_FRONTEND_IMAGE=${ARO_HCP_FRONTEND_IMAGE} \
		-p FRONTEND_MI_CLIENT_ID="null" > "$${TMP_DEPLOY}";\
	az aks command invoke --resource-group ${RESOURCE_GROUP} --name ${CLUSTER_NAME} --command "kubectl delete -f $$(basename $${TMP_DEPLOY})" --file "$${TMP_DEPLOY}"

.PHONY: frontend clean image deploy undeploy deploy-private undeploy-private
