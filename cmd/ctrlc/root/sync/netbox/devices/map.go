package devices

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ctrlplanedev/cli/internal/api"
)

func mapDevice(device netboxDevice) api.ResourceProviderResource {
	metadata := map[string]string{}

	links := map[string]string{
		"Device": device.Url,
		"Display URL": device.DisplayURL,
		"Device Type": device.DeviceType.Url,
		"Role": device.Role.Url,
	}
	linksJSON, err := json.Marshal(links)
	if err == nil {
		metadata["ctrlplane/links"] = string(linksJSON)
	}

	metadata["netbox/id"] = strconv.Itoa(int(device.Id))
	metadata["netbox/asset-tag"] = device.AssetTag
	metadata["netbox/serial"] = device.Serial

	metadata["netbox/site-id"] = fmt.Sprintf("%d", device.Site.Id)
	metadata["netbox/site-name"] = device.Site.Name

	metadata["netbox/device-type"] = device.DeviceType.Display
	metadata["netbox/device-type-id"] = fmt.Sprintf("%d", device.DeviceType.Id)
	metadata["netbox/device-type-url"] = device.DeviceType.Url
	metadata["netbox/device-type-slug"] = device.DeviceType.Slug
	metadata["netbox/device-type-manufacturer"] = device.DeviceType.Manufacturer.Name
	metadata["netbox/device-type-manufacturer-id"] = fmt.Sprintf("%d", device.DeviceType.Manufacturer.Id)
	metadata["netbox/device-type-manufacturer-url"] = device.DeviceType.Manufacturer.Url
	metadata["netbox/device-type-manufacturer-slug"] = device.DeviceType.Manufacturer.Slug
	metadata["netbox/device-type-model"] = device.DeviceType.Model

	metadata["netbox/role-name"] = device.Role.Name
	metadata["netbox/role-id"] = fmt.Sprintf("%d", device.Role.Id)
	metadata["netbox/role-url"] = device.Role.Url
	metadata["netbox/role-slug"] = device.Role.Slug

	if device.Status != nil {
		metadata["netbox/status"] = device.Status.Value
	}
	if device.Rack != nil {
		metadata["netbox/rack"] = device.Rack.Name
	}
	if device.Tenant != nil {
		metadata["netbox/tenant"] = device.Tenant.Name
	}
	if device.Role.Name != "" {
		metadata["netbox/role"] = device.Role.Name
	}
	if device.Platform != nil {
		metadata["netbox/platform"] = device.Platform.Name
		metadata["netbox/platform-id"] = fmt.Sprintf("%d", device.Platform.Id)
		metadata["netbox/platform-url"] = device.Platform.Url
		metadata["netbox/platform-slug"] = device.Platform.Slug
		metadata["netbox/platform-display"] = device.Platform.Display
		metadata["netbox/platform-device-count"] = fmt.Sprintf("%d", device.Platform.DeviceCount)
		metadata["netbox/platform-virtual-machine-count"] = fmt.Sprintf("%d", device.Platform.VirtualMachineCount)
	}

	for _, tag := range device.Tags {
		metadata[fmt.Sprintf("tags/%s", tag.Slug)] = "true"
	}

	config := make(map[string]any)
	bytes, _ := json.Marshal(device)
	_ = json.Unmarshal(bytes, &config)

	var name string
	if device.Name != nil && *device.Name != "" {
		name = *device.Name
	} else if device.DeviceType.Display != "" {
		name = device.DeviceType.Display
	} else if device.Serial != "" {
		name = device.Serial
	} else if device.Id != 0 {
		name = fmt.Sprintf("device-%d", device.Id)
	} else {
		name = "unknown-device"
	}

	identifier := strings.ReplaceAll(device.Url, "https://", "")
	identifier = strings.ReplaceAll(identifier, "http://", "")

	return api.ResourceProviderResource{
		Version:    "netbox/device/v1",
		Kind:       "Device",
		Name:       name,
		Identifier: identifier,
		Config:     config,
		Metadata:   metadata,
	}
}
