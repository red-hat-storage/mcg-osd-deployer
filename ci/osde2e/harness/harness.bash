#!/bin/bash

# Exit on errors and print verbose execution information.
set -ex

# Set default locations if not available in the environment.
CLUSTER_DIR="${CLUSTER_DIR:=/opt/cluster}" \
CLUSTER_KUBECONFIG="${CLUSTER_DIR}/auth/kubeconfig" \
OUTPUT_DIR="${OUTPUT_DIR:=/test-run-results}" \

# Setup Kubeconfig.
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
oc config set-cluster mcg-osd-test \
    --server=https://kubernetes.default.svc \
    --certificate-authority="${SERVICEACCOUNT}/ca.crt" \
    --embed-certs
oc config set-credentials cluster-admin \
    --token="$(<${SERVICEACCOUNT}/token)"
oc config set-context mcg-osd-test \
    --cluster mcg-osd-test \
    --user cluster-admin
oc config use-context mcg-osd-test
export KUBECONFIG="${HOME}/.kube/config"

# Populate cluster kubeconfig.
mkdir -pv "${CLUSTER_DIR}"/{auth,logs}
cp -v "$KUBECONFIG" "$CLUSTER_KUBECONFIG"

# Expected location for output file.
JUNIT_XML="${OUTPUT_DIR}/junit"

# Wait for the deployer CSV to come up.
while true; do
	# shellcheck disable=SC2207
	csvs=($(oc get csv -n redhat-data-federation -o jsonpath='{.items[*].metadata.name}')) 2>/dev/null
	for csv in "${csvs[@]}"; do
		if [[ $csv == mcg-osd-deployer* ]]; then
			deployer_csv=$csv
			break
		fi
	done
	[[ -n "$deployer_csv" ]] && break
	echo "Waiting for deployer CSV to be available..."
	sleep 30
done

# Wait for the deployer CSV to go into "Succeeded" state.
while true; do
	status=$(oc get csv "$deployer_csv" -n redhat-data-federation -o jsonpath='{.status.phase}') 2>/dev/null
	if [[ $status == "Succeeded" ]] || [[ $status == "Ready" ]]; then
		break
	fi
	echo "Waiting for the deployer to be ready..."
	sleep 60
done

# Hide.
set +x

# Login to OCM.
ENC_64="\
ZXlKaGJHY2lPaUpJVXpJMU5pSXNJblI1Y0NJZ09pQWlTbGRVSWl3aWEybGtJaUE2SUNKaFpEVXlN\
amRoTXkxaVkyWmtMVFJqWmpBdFlUZGlOaTB6T1RrNE16VmhNRGcxTmpZaWZRLmV5SnBZWFFpT2pF\
Mk5UZ3pNRGd5TmpVc0ltcDBhU0k2SW1SaU0yRTNOR0ZrTFRjd1pHWXRORFZrWVMwNE1HSXdMVEZs\
WWpRNU9URXhZVEU0TXlJc0ltbHpjeUk2SW1oMGRIQnpPaTh2YzNOdkxuSmxaR2hoZEM1amIyMHZZ\
WFYwYUM5eVpXRnNiWE12Y21Wa2FHRjBMV1Y0ZEdWeWJtRnNJaXdpWVhWa0lqb2lhSFIwY0hNNkx5\
OXpjMjh1Y21Wa2FHRjBMbU52YlM5aGRYUm9MM0psWVd4dGN5OXlaV1JvWVhRdFpYaDBaWEp1WVd3\
aUxDSnpkV0lpT2lKbU9qVXlPR1EzTm1abUxXWTNNRGd0TkRObFpDMDRZMlExTFdabE1UWm1OR1ps\
TUdObE5qcHdjbUZ6Y21sMllTMXpkRzl5WVdkbExXOWpjeUlzSW5SNWNDSTZJazltWm14cGJtVWlM\
Q0poZW5BaU9pSmpiRzkxWkMxelpYSjJhV05sY3lJc0ltNXZibU5sSWpvaU5qTmhZMkZsTW1VdFpU\
aGpOeTAwWW1WbExUaGlOall0WVRjd1pqUTNPR0ZsTURCaklpd2ljMlZ6YzJsdmJsOXpkR0YwWlNJ\
NklqazFPR1JtTVdJNExXRmlabUV0TkdJNE1TMDVNMlF6TFRJNE1USmtNVGxtTmpRd09DSXNJbk5q\
YjNCbElqb2liM0JsYm1sa0lHRndhUzVwWVcwdWMyVnlkbWxqWlY5aFkyTnZkVzUwY3lCdlptWnNh\
VzVsWDJGalkyVnpjeUlzSW5OcFpDSTZJamsxT0dSbU1XSTRMV0ZpWm1FdE5HSTRNUzA1TTJRekxU\
STRNVEprTVRsbU5qUXdPQ0o5LmtBMHFIQ1Y1MmRqSWZvblc5YzR3Mk9hR2UtZ1RTTlBoNEg5TWN3\
OHEwc00K\
"

ocm login --token="$(echo $ENC_64 | base64 -d)" --url=staging

# Show.
set -x

# List all discovered clusters.
ocm list clusters

# Get current cluster.
CLUSTER_ID="$(ocm list cluster | grep -w osde2e | tail -n1 | awk 'NR==1{print $1}')"

# Exit with error if no cluster found.
if [[ -z "$CLUSTER_ID" ]]; then
  echo "No cluster found, exiting..."
  exit 1
fi

# Required by Cypress.
ARTIFACTS_DIRECTORY="/logs/artifacts"
export ARTIFACTS_DIRECTORY

# Required by Cypress.
BRIDGE_BASE_ADDRESS="$(ocm describe cluster "${CLUSTER_ID}" | grep -w "Console URL" | awk '{print $3}')"
BRIDGE_KUBEADMIN_PASSWORD="$(ocm get /api/clusters_mgmt/v1/clusters/"${CLUSTER_ID}"/credentials | jq -r .admin.password)"
export BRIDGE_BASE_ADDRESS
export BRIDGE_KUBEADMIN_PASSWORD

# exit if BRIDGE_BASE_ADDRESS is empty
if [[ -z "$BRIDGE_BASE_ADDRESS" ]]; then
  echo "No console address found, exiting..."
  exit 1
fi

# exit if BRIDGE_KUBEADMIN_PASSWORD is empty
if [[ -z "$BRIDGE_KUBEADMIN_PASSWORD" ]]; then
  echo "No password found, exiting..."
  exit 1
fi

# Required by Cypress.
CYPRESS_RUNNER="osde2e"
export CYPRESS_RUNNER

# Clusters created have this as their username by default.
# This is same as what we are using in the UI plugin's test suite, but setting it here is a good practice, nonetheless.
BRIDGE_HTPASSWD_USERNAME="kubeadmin"
BRIDGE_E2E_BROWSER_NAME="electron"
export BRIDGE_HTPASSWD_USERNAME
export BRIDGE_E2E_BROWSER_NAME

# Setup UI plugin.
git clone https://github.com/red-hat-storage/mcg-ms-console &&
	cd mcg-ms-console &&
	yarn install --production=false &&

	# Run Cypress tests *selectively*.
	yarn run test-cypress-headless --spec 'cypress/tests/!(bucket-policy|resource-provider-card|status-card|data-source|inventory-card).spec.ts' &&
	yarn run cypress-postreport &&

	mv cypress-gen/*.xml /test-run-results/

	# # XML string to be injected.
	# ISTR="<?xml version=\"1.0\" encoding=\"UTF-8\"?>"

	# # Remove all XML meta tags from all generated JUnit files.
	# sed -i 's/<?xml.*>//g' cypress-gen/*.xml

	# # Introduce a meta XML tag at the beginning.
	# echo "$ISTR" > "$JUNIT_XML" 

	# # Start super tag.
	# echo "<root>" >> "$JUNIT_XML" 

	# # Append the rest of the concatenated XMLs.
	# cat cypress-gen/*.xml >> "$JUNIT_XML"

	# # Close super tag.
	# echo "</root>" >> "$JUNIT_XML" 

	# # Store JUnit outputs in the expected location.
	# mv "$JUNIT_XML" "$JUNIT_XML".xml
