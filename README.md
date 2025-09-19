## Configuring the provider

```toml
namespace = "garm-runners"

[credentials]
    kubeconfig = "/etc/kubeconfig/kubeconfig.yaml"
```

## Tweaking the provider

```json

{
    "$schema": "http://cloudbase.it/garm-provider-harvester/schemas/extra_specs#",
    "type": "object",
    "description": "Schema defining supported extra specs for the Garm Harvester Provider",
    "properties": {
        "network_id": {
            "type": "string",
            "description": "The tenant network to which runners will be connected to."
        },
        "boot_disk_size": {
            "type": "integer",
            "description": "The size of the root disk in GB. Default is 50 GB."
        },
        "disable_updates": {
            "type": "boolean",
            "description": "Disable automatic updates on the VM."
        },
        "extra_packages": {
            "type": "array",
            "description": "Extra packages to install on the VM.",
            "items": {
                "type": "string"
            }
        },
        "runner_install_template": {
            "type": "string",
            "description": "This option can be used to override the default runner install template. If used, the caller is responsible for the correctness of the template as well as the suitability of the template for the target OS. Use the extra_context extra spec if your template has variables in it that need to be expanded."
        },
        "extra_context": {
            "type": "object",
            "description": "Extra context that will be passed to the runner_install_template.",
            "additionalProperties": {
                "type": "string"
            }
        },
        "pre_install_scripts": {
            "type": "object",
            "description": "A map of pre-install scripts that will be run before the runner install script. These will run as root and can be used to prep a generic image before we attempt to install the runner. The key of the map is the name of the script as it will be written to disk. The value is a byte array with the contents of the script.",
            "additionalProperties": {
                "type": "string"
            }
        }
    },
	"additionalProperties": false
}
```