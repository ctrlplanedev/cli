package devices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

func fetchAllDevicesDirect(ctx context.Context, netboxURL, netboxToken string, filters deviceFilters) ([]netboxDevice, error) {
	var all []netboxDevice
	var offset int32
	page := 1
	base := strings.TrimRight(netboxURL, "/") + "/api/dcim/devices/"
	httpClient := &http.Client{Timeout: 45 * time.Second}

	for {
		log.Info(
			"Fetching Netbox devices page",
			"page", page,
			"offset", offset,
			"limit", pageSize,
			"q", filters.Query,
			"site", filters.Site,
			"role", filters.Role,
			"status", filters.Status,
			"status_n", filters.StatusExclude,
			"tag", filters.Tag,
			"tenant", filters.Tenant,
		)

		res, err := fetchDevicesPage(ctx, httpClient, base, netboxToken, filters.toQuery(offset))
		if err != nil {
			log.Error(err, "Failed to fetch Netbox devices page", "page", page, "offset", offset)
			return nil, err
		}

		log.Info("Fetched devices from Netbox page", "page", page, "count", len(res.Results), "total", res.Count)
		all = append(all, res.Results...)

		if res.Next == nil || *res.Next == "" || int32(len(all)) >= res.Count {
			log.Info("All Netbox devices fetched", "total_count", len(all))
			break
		}
		offset += pageSize
		page++
	}

	return all, nil
}

func fetchDevicesPage(
	ctx context.Context,
	httpClient *http.Client,
	baseURL string,
	netboxToken string,
	queryParams url.Values,
) (netboxListResponse, error) {
	endpoint := baseURL + "?" + queryParams.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return netboxListResponse{}, fmt.Errorf("failed to build Netbox request: %w", err)
	}
	req.Header.Set("Authorization", "Token "+netboxToken)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return netboxListResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return netboxListResponse{}, fmt.Errorf(
			"netbox /api/dcim/devices request failed (HTTP %d): %s",
			resp.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var parsed netboxListResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return netboxListResponse{}, fmt.Errorf("failed to decode Netbox devices response: %w", err)
	}

	return parsed, nil
}
