package devices

type netboxListResponse struct {
	Count    int32          `json:"count"`
	Next     *string        `json:"next"`
	Previous *string        `json:"previous"`
	Results  []netboxDevice `json:"results"`
}

type netboxNamedRef struct {
	Id   int32  `json:"id"`
	Url  string `json:"url"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type netboxStatus struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type netboxChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type netboxSimpleNamed struct {
	Name string `json:"name"`
}

type netboxPlatform struct {
	Id                  int32   `json:"id"`
	Url                 string  `json:"url"`
	Display             string  `json:"display"`
	Name                string  `json:"name"`
	Slug                string  `json:"slug"`
	Description         *string `json:"description"`
	DeviceCount         int32   `json:"device_count"`
	VirtualMachineCount int32   `json:"virtualmachine_count"`
}

type netboxTag struct {
	Slug string `json:"slug"`
}

type netboxPrimaryIP struct {
	Address string `json:"address"`
}

type netboxDeviceType struct {
	Id           int32          `json:"id"`
	Url          string         `json:"url"`
	Display      string         `json:"display"`
	Manufacturer netboxNamedRef `json:"manufacturer"`
	Model        string         `json:"model"`
	Slug         string         `json:"slug"`
	Description  *string        `json:"description"`
	DeviceCount  int32          `json:"device_count"`
}

type netboxCustomASN struct { //nolint:unused
	ASN         int64  `json:"asn"`
	Description string `json:"description"`
	Display     string `json:"display"`
	Id          int32  `json:"id"`
	Url         string `json:"url"`
}

type netboxDevice struct {
	Id           int32              `json:"id"`
	Url          string             `json:"url"`
	DisplayURL   string             `json:"display_url"`
	Display      string             `json:"display"`
	Name         *string            `json:"name"`
	DeviceType   netboxDeviceType   `json:"device_type"`
	Role         netboxNamedRef     `json:"role"`
	Tenant       *netboxSimpleNamed `json:"tenant"`
	Platform     *netboxPlatform    `json:"platform"`
	Serial       string             `json:"serial"`
	AssetTag     string             `json:"asset_tag"`
	Site         netboxNamedRef     `json:"site"`
	Location     *netboxSimpleNamed `json:"location"`
	Rack         *netboxSimpleNamed `json:"rack"`
	Status       *netboxStatus      `json:"status"`
	Airflow      *netboxChoice      `json:"airflow"`
	PrimaryIP    *netboxPrimaryIP   `json:"primary_ip"`
	PrimaryIP4   *netboxPrimaryIP   `json:"primary_ip4"`
	PrimaryIP6   *netboxPrimaryIP   `json:"primary_ip6"`
	OOBIP        *netboxPrimaryIP   `json:"oob_ip"`
	Description  string             `json:"description"`
	Comments     string             `json:"comments"`
	Tags         []netboxTag        `json:"tags"`
	CustomFields map[string]any     `json:"custom_fields"`
}
