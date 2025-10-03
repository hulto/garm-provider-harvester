#!/bin/bash
export GARM_PROVIDER_CONFIG_FILE="./test-providerconfig.toml"
export GARM_CONTROLLER_ID="1ce5d837-9d7a-4860-a05a-d29c30197673"
export GARM_POOL_ID="35615b31-0029-4023-9b09-adb95b91da90"

RUNNER_IMAGE="harvester-public/ubuntu-server-noble-24.04"
RUNNER_IMAGE="harvester-public/windows-2025-runner"
RUNNER_STORAGECLASS="longhorn-ubuntu-server-noble-24.04"
RUNNER_STORAGECLASS="longhorn-windows-2025-runner"
OS_TYPE="windows"
FLAVOR="custom-4c-16Gi-164Gi"
RUNNER_NAME="garm-FcsOAwWBYJmL"
TEST_STDIN_PATH="./test-createinstance-stdin.json"
cat << EOF > $TEST_STDIN_PATH
{
  "name": "$RUNNER_NAME",
  "tools": [
    {
      "os": "osx",
      "architecture": "x64",
      "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-osx-x64-2.299.1.tar.gz",
      "filename": "actions-runner-osx-x64-2.299.1.tar.gz",
      "sha256_checksum": "b0128120f2bc48e5f24df513d77d1457ae845a692f60acf3feba63b8d01a8fdc"
    },
    {
      "os": "linux",
      "architecture": "x64",
      "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-linux-x64-2.299.1.tar.gz",
      "filename": "actions-runner-linux-x64-2.299.1.tar.gz",
      "sha256_checksum": "147c14700c6cb997421b9a239c012197f11ea9854cd901ee88ead6fe73a72c74"
    },
    {
      "os": "windows",
      "architecture": "x64",
      "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-win-x64-2.299.1.zip",
      "filename": "actions-runner-win-x64-2.299.1.zip",
      "sha256_checksum": "f7940b16451d6352c38066005f3ee6688b53971fcc20e4726c7907b32bfdf539"
    },
    {
      "os": "linux",
      "architecture": "arm",
      "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-linux-arm-2.299.1.tar.gz",
      "filename": "actions-runner-linux-arm-2.299.1.tar.gz",
      "sha256_checksum": "a4d66a766ff3b9e07e3e068a1d88b04e51c27c9b94ae961717e0a5f9ada998e6"
    },
    {
      "os": "linux",
      "architecture": "arm64",
      "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-linux-arm64-2.299.1.tar.gz",
      "filename": "actions-runner-linux-arm64-2.299.1.tar.gz",
      "sha256_checksum": "debe1cc9656963000a4fbdbb004f475ace5b84360ace2f7a191c1ccca6a16c00"
    },
    {
      "os": "osx",
      "architecture": "arm64",
      "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-osx-arm64-2.299.1.tar.gz",
      "filename": "actions-runner-osx-arm64-2.299.1.tar.gz",
      "sha256_checksum": "f73849b9a78459d2e08b9d3d2f60464a55920de120e228b0645b01abe68d9072"
    },
    {
      "os": "windows",
      "architecture": "arm64",
      "download_url": "https://github.com/actions/runner/releases/download/v2.299.1/actions-runner-win-arm64-2.299.1.zip",
      "filename": "actions-runner-win-arm64-2.299.1.zip",
      "sha256_checksum": "d1a9d8209f03589c8dc05ee17ae8d194756377773a4010683348cdd6eefa2da7"
    }
  ],
  "repo_url": "https://github.com/gabriel-samfira/scripts",
  "callback-url": "https://garm.example.com/api/v1/callbacks",
  "metadata-url": "https://garm.example.com/api/v1/metadata",
  "instance-token": "super secret JWT token",
  "extra_specs": {
    "network_name": "harvester-public/harvester-public-net",
    "network_adapter_type": "e1000",
    "network_type": "bridge",
    "disk_connector_type": "sata"
  },
  "ssh-keys": [
    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIMOTygNEK4LTfZwV1Pqf9vX5AECGXDe3paaFhiJsJvUU hulto@axe.local"
  ],
  "ca-cert-bundle": null,
  "github-runner-group": "my_group",
  "os_type": "$OS_TYPE",
  "arch": "amd64",
  "flavor": "$FLAVOR",
  "image": "$RUNNER_IMAGE",
  "labels": [
    "ubuntu",
    "openstack",
    "runner-controller-id:f9286791-1589-4f39-a106-5b68c2a18af4",
    "runner-pool-id:9dcf590a-1192-4a9c-b3e4-e0902974c2c0"
  ],
  "pool_id": "9dcf590a-1192-4a9c-b3e4-e0902974c2c0"
}
EOF

test_create_vm() {
    export GARM_COMMAND="CreateInstance"

    cat $TEST_STDIN_PATH | go run . && echo "[+] Create Instance Okay"
    unset GARM_COMMAND
}

test_list_vm() {
    export GARM_COMMAND="ListInstances"

    go run . && echo "[+] List Instance Okay"

    unset GARM_COMMAND
}

test_delete_vm() {
    export GARM_COMMAND="DeleteInstance"
    export GARM_INSTANCE_ID=$RUNNER_NAME
    go run . && echo "[+] Delete Instance Okay"

    unset GARM_COMMAND
    unset GARM_INSTANCE_ID
}

test_get_vm() {
    export GARM_COMMAND="GetInstance"
    export GARM_INSTANCE_ID=$RUNNER_NAME
    go run . && echo "[+] Get Instance Okay"

    unset GARM_COMMAND
    unset GARM_INSTANCE_ID
}

test_get_version() {
    export GARM_COMMAND="GetVersion"
    go run . && echo "[+] Get Version Okay"

    unset GARM_COMMAND
}

test_start_vm() {
    export GARM_COMMAND="StartInstance"
    export GARM_INSTANCE_ID=$RUNNER_NAME
    go run . && echo "[+] Start Instance Okay"

    unset GARM_COMMAND
    unset GARM_INSTANCE_ID
}

test_stop_vm() {
    export GARM_COMMAND="StopInstance"
    export GARM_INSTANCE_ID=$RUNNER_NAME
    go run . && echo "[+] Stop Instance Okay"

    unset GARM_COMMAND
    unset GARM_INSTANCE_ID
}

test_remove_all() {
    export GARM_COMMAND="RemoveAllInstances"
    go run . && echo "[+] Remove All Instances Okay"

    unset GARM_COMMAND
}

test_create_vm
test_get_vm
test_list_vm
test_start_vm
test_stop_vm
test_delete_vm

test_create_vm
test_remove_all
test_get_version

rm $TEST_STDIN_PATH

# docker run -v $(pwd)/test-providerconfig.toml:/etc/garm/garm-provider-harvester.toml:ro \
#     -v /etc/kubeconfig:/etc/kubeconfig:ro \
#     -v $(pwd)/garm-config.toml:/etc/garm/config.toml:ro \
#     -it ghcr.io/hulto/garm-provider-harvester:0.0.c


# BACKING_IMAGE=$(./kubectl get backingimages.longhorn.io -n longhorn-system -o jsonpath='{.items[?(@.metadata.annotations.harvesterhci\.io\/imageId == "harvester-public/ubuntu-server-noble-24.04")].metadata.name}')

# ./kubectl get storageclass -o jsonpath='{.items[?(@.parameters.backingImage == "'$BACKING_IMAGE'")].metadata.name}'
