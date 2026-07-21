#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_DIR="$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

DOCKER="${DOCKER:-docker}"
# OCM must point at the OCM CLI (module ocm.software/open-component-model/cli).
# The Makefile builds it into bin/ocm.
OCM="${OCM:-ocm}"
GO="${GO:-go}"

VERSION="${VERSION:-"0.0.$(date +%s)"}"
KEEP_ZOT=false

# Parse arguments
while [[ $# -gt 0 ]]; do
	case $1 in
    -h|--help)
      echo "e2e.sh - runs e2e tests"
      echo " "
      echo "./e2e.sh [options]"
      echo " "
      echo "options:"
      echo "-h, --help           show brief help"
      echo "--keep-zot           keeps zot running after running tests"
      echo "--version VERSION    specify component version created during test"
      exit 0
      ;;
		--keep-zot)
			KEEP_ZOT=true
			shift
			;;
		--version)
			VERSION="$2"
			shift 2
			;;
		*)
			shift
			;;
	esac
done

# Check if zot is already running
if ! ${DOCKER} ps | grep -q zot-registry; then
	echo "Starting zot registry..."
	${DOCKER} run -d -p 5000:5000 \
		  --name zot-registry \
		  -v "${SCRIPT_DIR}/fixtures/zot-config.json:/etc/zot/config.json:ro" \
		  -v zot-data:/var/lib/registry \
		  ghcr.io/project-zot/zot:v2.1.10
else
	echo "zot registry already running"
fi

# ---------------------------------------------------------------------------
# OCM CLI usage
#
# Build a CTF and transfer it to the registry:
#   ocm add component-version --repository ./ctf --constructor component-constructor.yaml
#   ocm transfer component-version ctf::./ctf//<component>:<version> <target> \
#       --copy-resources --upload-as ociArtifact
#
# Notes:
#   * `add component-version` has no --version flag; the component version comes
#     from the constructor. It is injected via environment-variable substitution
#     (COMPONENT_VERSION, exported below) which the CLI expands in the constructor.
#   * The helm-values-template file input needs an explicit OCI-compliant
#     mediaType in the constructor (application/x-yaml); see component-constructor.yaml.
#   * The transfer source is a full component reference (ctf::<dir>//<name>:<ver>),
#     not just the CTF directory.
# ---------------------------------------------------------------------------

COMPONENT="opendefense.cloud/arc"
CONSTRUCTOR="component-constructor.yaml"

# Check if CTF needs to be created and transferred
CTF_DIR="${SCRIPT_DIR}/fixtures/arc/ctf"
ARTIFACT_INDEX="${CTF_DIR}/artifact-index.json"

if [ ! -f "$ARTIFACT_INDEX" ] || ! grep -q "\"tag\":\"${VERSION}\"" "$ARTIFACT_INDEX"; then
	echo "Creating and transferring component version ${VERSION}..."
	rm -rf "${CTF_DIR}"
	(cd "$SCRIPT_DIR/fixtures/arc" && COMPONENT_VERSION="${VERSION}" ${OCM} add component-version --repository ./ctf --constructor "${CONSTRUCTOR}")
	# Absolute access: resources are uploaded as OCI artifacts, so their access
	# carries an absolute imageReference pointing at the target registry.
	${OCM} transfer component-version "ctf::${CTF_DIR}//${COMPONENT}:${VERSION}" http://localhost:5000/my-components --copy-resources --upload-as ociArtifact
else
	echo "Component version ${VERSION} already exists in CTF"
fi

echo "Running ocm-kit CLI with component version ${VERSION}..."

# Test 1: Render with default template
echo "Test 1: Rendering default Helm values template..."
OUTPUT1=$(${GO} run cmd/ocm-kit/main.go "http://localhost:5000/my-components//opendefense.cloud/arc:${VERSION}" -r helm-chart)
if echo "$OUTPUT1" | grep -q "apiserver:" && \
   echo "$OUTPUT1" | grep -q "controller:" && \
   echo "$OUTPUT1" | grep -q "etcd:" && \
   echo "$OUTPUT1" | grep -q "localhost:5000/my-components/opendefensecloud/arc-apiserver" && \
   echo "$OUTPUT1" | grep -q "localhost:5000/my-components/opendefensecloud/arc-controller-manager" && \
   echo "$OUTPUT1" | grep -q "localhost:5000/my-components/coreos/etcd"; then
	echo "✓ Test 1 passed: Default template rendered correctly"
else
	echo "✗ Test 1 failed: Default template output missing expected content"
	echo "Output was:"
	echo "$OUTPUT1"
	exit 1
fi

# Test 2: Render with override template
echo "Test 2: Rendering override Helm values template..."
OUTPUT2=$(${GO} run cmd/ocm-kit/main.go "http://localhost:5000/my-components//opendefense.cloud/arc:${VERSION}" -r helm-chart --local-helm-values-template "$SCRIPT_DIR/fixtures/arc/override-values.yaml.tpl")
if echo "$OUTPUT2" | grep -q "foobar:" && \
   echo "$OUTPUT2" | grep -q "fizzbuzz:" && \
   echo "$OUTPUT2" | grep -q "helloworld:" && \
   echo "$OUTPUT2" | grep -q "localhost:5000/my-components/opendefensecloud/arc-apiserver" && \
   echo "$OUTPUT2" | grep -q "localhost:5000/my-components/opendefensecloud/arc-controller-manager" && \
   echo "$OUTPUT2" | grep -q "localhost:5000/my-components/coreos/etcd"; then
	echo "✓ Test 2 passed: Override template rendered correctly"
else
	echo "✗ Test 2 failed: Override template output missing expected content"
	echo "Output was:"
	echo "$OUTPUT2"
	exit 1
fi

# ---------------------------------------------------------------------------
# Tests 3 & 4: relative / local access.
#
# `--upload-as localBlob` copies resources into the target registry as
# LocalBlobs, each carrying only a repository-relative `referenceName`
# (e.g. "opendefensecloud/arc-apiserver:v0.2.0") with no registry host.
#
# The ocm-kit CLI reconstructs the full, host-qualified reference from the
# repository context it holds — the base URL it opened the repo with,
# "<host>/<namespace>" (${REL_ACCESS_HOST}/my-components) derived from the
# component-version reference — by prefixing it to the host-less referenceName.
# The rendered refs are therefore identical to Tests 1 & 2, just reached via the
# relative-access component.
# ---------------------------------------------------------------------------
REL_VERSION="${VERSION}-rel"
REL_CTF_DIR="${SCRIPT_DIR}/fixtures/arc/ctf-rel"
REL_ARTIFACT_INDEX="${REL_CTF_DIR}/artifact-index.json"
REL_ACCESS_HOST="${REL_ACCESS_HOST:-127.0.0.1:5000}"

if [ ! -f "$REL_ARTIFACT_INDEX" ] || ! grep -q "\"tag\":\"${REL_VERSION}\"" "$REL_ARTIFACT_INDEX"; then
	echo "Creating and transferring component version ${REL_VERSION} with relative (localBlob) access..."
	rm -rf "${REL_CTF_DIR}"
	(cd "$SCRIPT_DIR/fixtures/arc" && COMPONENT_VERSION="${REL_VERSION}" ${OCM} add component-version --repository ./ctf-rel --constructor "${CONSTRUCTOR}")
	# Relative/local access: resources are uploaded as LocalBlobs in the target.
	${OCM} transfer component-version "ctf::${REL_CTF_DIR}//${COMPONENT}:${REL_VERSION}" http://localhost:5000/my-components --copy-resources --upload-as localBlob
else
	echo "Component version ${REL_VERSION} already exists in CTF"
fi

# Test 3: Render default template with relative (localBlob) access
echo "Test 3: Rendering default Helm values template (relative/localBlob access)..."
OUTPUT3=$(${GO} run cmd/ocm-kit/main.go "http://${REL_ACCESS_HOST}/my-components//opendefense.cloud/arc:${REL_VERSION}" -r helm-chart)
if echo "$OUTPUT3" | grep -q "apiserver:" && \
   echo "$OUTPUT3" | grep -q "controller:" && \
   echo "$OUTPUT3" | grep -q "etcd:" && \
   echo "$OUTPUT3" | grep -Fq "${REL_ACCESS_HOST}/my-components/opendefensecloud/arc-apiserver" && \
   echo "$OUTPUT3" | grep -Fq "${REL_ACCESS_HOST}/my-components/opendefensecloud/arc-controller-manager" && \
   echo "$OUTPUT3" | grep -Fq "${REL_ACCESS_HOST}/my-components/coreos/etcd"; then
	echo "✓ Test 3 passed: Default template rendered correctly with relative access"
else
	echo "✗ Test 3 failed: Default template with relative access output missing expected content"
	echo "Output was:"
	echo "$OUTPUT3"
	exit 1
fi

# Test 4: Render override template with relative (localBlob) access
echo "Test 4: Rendering override Helm values template (relative/localBlob access)..."
OUTPUT4=$(${GO} run cmd/ocm-kit/main.go "http://${REL_ACCESS_HOST}/my-components//opendefense.cloud/arc:${REL_VERSION}" -r helm-chart --local-helm-values-template "$SCRIPT_DIR/fixtures/arc/override-values.yaml.tpl")
if echo "$OUTPUT4" | grep -q "foobar:" && \
   echo "$OUTPUT4" | grep -q "fizzbuzz:" && \
   echo "$OUTPUT4" | grep -q "helloworld:" && \
   echo "$OUTPUT4" | grep -Fq "${REL_ACCESS_HOST}/my-components/opendefensecloud/arc-apiserver" && \
   echo "$OUTPUT4" | grep -Fq "${REL_ACCESS_HOST}/my-components/opendefensecloud/arc-controller-manager" && \
   echo "$OUTPUT4" | grep -Fq "${REL_ACCESS_HOST}/my-components/coreos/etcd"; then
	echo "✓ Test 4 passed: Override template rendered correctly with relative access"
else
	echo "✗ Test 4 failed: Override template with relative access output missing expected content"
	echo "Output was:"
	echo "$OUTPUT4"
	exit 1
fi

# Test 5: Render with pull secrets file
echo "Test 5: Rendering with pull secrets file..."
OUTPUT5=$(${GO} run cmd/ocm-kit/main.go "http://localhost:5000/my-components//opendefense.cloud/arc:${VERSION}" \
  --local-helm-values-template "$SCRIPT_DIR/fixtures/arc/pull-secrets-values.yaml.tpl" \
  --pull-secrets-file "$SCRIPT_DIR/fixtures/arc/pull-secrets.json")
if [ "$(echo "$OUTPUT5" | grep -c -- "- name: regcred")" -eq 3 ] && \
   [ "$(echo "$OUTPUT5" | grep -c -- "imagePullSecrets:")" -eq 3 ]; then
	echo "✓ Test 5 passed: Pull secrets rendered correctly"
else
	echo "✗ Test 5 failed: Pull secrets output missing expected content"
	echo "Output was:"
	echo "$OUTPUT5"
	exit 1
fi

# Cleanup only if --keep-zot was not provided
if [ "$KEEP_ZOT" = false ]; then
	echo "Stopping zot registry..."
	${DOCKER} stop zot-registry
	${DOCKER} rm -f zot-registry
	${DOCKER} volume rm zot-data
else
	echo "Keeping zot registry running (--keep-zot flag provided)"
fi
