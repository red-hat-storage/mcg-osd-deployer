#!/bin/bash

set -e

# Set default locations if not available in the environment.
# These are configured in the Dockerfile by default.
CLUSTER_DIR="${CLUSTER_DIR:=/opt/cluster}"
CLUSTER_KUBECONFIG="${CLUSTER_DIR}/auth/kubeconfig"
OCSCI_INSTALL_DIR="${OCSCI_INSTALL_DIR:=/opt/ocs-ci}"
OUTPUT_DIR="${OUTPUT_DIR:=/test-run-results}"
CLUSTER_CONFIG="${CLUSTER_CONFIG:=${OCSCI_INSTALL_DIR}/conf/ocsci/dfms.yaml}"

# Location of the junit output file.
JUNIT_XML="${OUTPUT_DIR}/junit.xml"

# Create kubeconfig.
echo "### Setting up kubeconfig."
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
echo -n "- " && \
  kubectl config set-cluster mcg-osd-test \
    --server=https://kubernetes.default.svc \
    --certificate-authority="${SERVICEACCOUNT}/ca.crt" \
    --embed-certs
echo -n "- " && \
  kubectl config set-credentials cluster-admin --token="$(<${SERVICEACCOUNT}/token)"
echo -n "- " && \
  kubectl config set-context mcg-osd-test \
    --cluster mcg-osd-test \
    --user cluster-admin
echo -n "- " && \
  kubectl config use-context mcg-osd-test

export KUBECONFIG="${HOME}/.kube/config"
KUBE_CLUSTER_NAME="mcg-osd-test"

# Debug output to stdout to be captured into artifacts by the harness.
echo "### kubeconfig content in '$KUBECONFIG'."
echo
cat "$KUBECONFIG"
echo

# Copy kubeconfig to cluster directory.
echo "### Setting up the cluster directory."
echo
echo -n "- " && mkdir -pv "${CLUSTER_DIR}"/{auth,logs}
echo -n "- " && cp -v "$KUBECONFIG" "$CLUSTER_KUBECONFIG"
echo

echo "### Checking deployer to be ready"
while true; do
	# shellcheck disable=SC2207
	csvs=(`kubectl get csv -n redhat-data-federation -o jsonpath='{.items[*].metadata.name}'`) 2> /dev/null
	for csv in "${csvs[@]}"
	do
		if  [[ $csv == mcg-osd-deployer* ]]
			then 
				deployerCsv=$csv
				break
			fi
	done
	[[  -n "$deployerCsv" ]] && break
	echo "Waiting for deployer csv to be available"
	sleep 30
done

while true; do
  status=`kubectl get csv "$deployerCsv" -n redhat-data-federation -o jsonpath='{.status.phase}'` 2> /dev/null
	if [[ $status == "Succeeded" ]] || [[ $status == "Ready" ]]
		then
			break
	fi
	echo "Waiting for the deployer to be ready"
	sleep 60
done

sleep 480

# Run Cypress tests.
git clone https://github.com/red-hat-storage/mcg-ms-console
cd mcg-ms-console
yarn install --production=false
yarn run test-cypress-headless
yarn run cypress-postreport
cat cypress-gen/*.xml > out.xml

# Remove testsuites tag from junit xml
sed -i 's/<testsuites>\|<\/testsuites>//g' $JUNIT_XML
