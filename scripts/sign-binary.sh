#!/bin/bash
set -e

artifact="$1"
target="$2"

if [ -z "$artifact" ] || [ -z "$target" ]; then
	echo "Usage: $0 <artifact_path> <target>"
	exit 1
fi

echo "Processing artifact: $artifact (target: $target)"

# Determine OS from target (format: os_arch, e.g., darwin_amd64, windows_386)
os=$(echo "$target" | cut -d'_' -f1)

# Sign based on OS
if [ "$os" = "windows" ]; then
	echo "Signing Windows binary: $artifact"
	docker run \
		--rm \
		-e "GCLOUD_PROJECT=${GCLOUD_PROJECT}" \
		-e "GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS}" \
		-v "${HOST_PWD}/.gitlab-secrets:/var/run/secrets/gitlab" \
		-v "${HOST_PWD}:/work" \
		registry.gitlab.com/gitlab-com/gl-infra/common-ci-tasks-images/code-signer:1.3.0 \
		sign-windows-binaries --overwrite "/work/$artifact"

elif [ "$os" = "darwin" ]; then
	echo "Signing macOS binary: $artifact"
	docker run \
		--rm \
		-e "APPSTORE_CONNECT_API_KEY_FILE=${APPSTORE_CONNECT_API_KEY_FILE}" \
		-e "GCLOUD_PROJECT=${GCLOUD_PROJECT}" \
		-e "GOOGLE_APPLICATION_CREDENTIALS=${GOOGLE_APPLICATION_CREDENTIALS}" \
		-v "${HOST_PWD}/.gitlab-secrets:/var/run/secrets/gitlab" \
		-v "${HOST_PWD}:/work" \
		-v "${APPSTORE_CONNECT_API_KEY_FILE}:${APPSTORE_CONNECT_API_KEY_FILE}" \
		registry.gitlab.com/gitlab-com/gl-infra/common-ci-tasks-images/code-signer:1.3.0 \
		sign-macos-binaries --overwrite "/work/$artifact"

else
	echo "Not a Windows or macOS binary, no code signing needed for: $os"
	exit 0
fi

echo "Signing complete!"
ls -al "$artifact"
