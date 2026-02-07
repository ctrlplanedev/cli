package common

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
)

type ResourceGroupInfo struct {
	Name     string
	Location string
}

func GetResourceGroupInfo(
	ctx context.Context, cred azcore.TokenCredential, subscriptionID string,
) ([]ResourceGroupInfo, error) {

	results := make([]ResourceGroupInfo, 0)

	client, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Resource Group client: %w", err)
	}
	pager := client.NewListPager(nil)
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get next page of resource groups: %w", err)
		}

		// Iterate through the resource groups in the current page
		for _, rg := range page.Value {
			results = append(results, ResourceGroupInfo{
				Name:     *rg.Name,
				Location: *rg.Location,
			})
		}
	}

	return results, nil
}
