package steampipe

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter"
	"github.com/ctrlplanedev/cli/cmd/ctrlc/root/sync/steampipe/adapter/model"
	"github.com/ctrlplanedev/cli/internal/api"
)

func (c *Client) DoSync(workspaceId string, table string) ([]api.AgentResource, error) {
	var jsResource string
	var ok bool

	ad := adapter.SelectAdapter(table)
	if ad == nil {
		return nil, fmt.Errorf("no adapter found for table %s", table)
	}

	resources := make([]api.AgentResource, 0)

	jsonRows, err := c.SelectAll(table)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from %s table: %w", table, err)
	}

	log.Infof("steampipe '%s' query returned %d rows", table, len(jsonRows))

	if len(jsonRows) == 0 {
		return nil, nil
	}

	var sqlRow model.SqlRow = model.SqlRow{
		EntityName: table,
		Json:       "",
	}

	for _, jsRow := range jsonRows {

		sqlRow.Json = jsRow

		log.Debugf("SqlRow '%s' JSON \n%s", table, jsRow)

		if jsResource, ok = ad.ToResourceJson(workspaceId, sqlRow); ok {
			if log.GetLevel() >= log.DebugLevel {
				payloadStr, _ := json.Marshal(jsResource)
				log.Debugf("Resource JSON \n%s", payloadStr)
			}
			var apiResource api.AgentResource
			if err = json.Unmarshal([]byte(jsResource), &apiResource); err != nil {
				log.Errorf("Failed to unmarshal json resource '%s': %s", jsResource, err)
			} else {
				resources = append(resources, apiResource)
			}
		}
	}

	return resources, nil
}

// SelectAll returns all foreign tables where foreign_table_catalog is 'steampipe'
func (c *Client) SelectAll(tableName string) ([]string, error) {
	var columns []string
	var err error

	results := make([]string, 0)

	query := fmt.Sprintf("SELECT * FROM %s", tableName)

	rows, err := c.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s table: %w", tableName, err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			log.Errorf("failed to close rows: %v", err)
		}
	}(rows)

	columns, err = rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns from %s table: %w", tableName, err)
	}

	var row map[string]interface{}
	var rowJson []byte
	for rows.Next() {
		if row, err = toJsonObj(rows, columns); err != nil {
			return nil, fmt.Errorf("failed to convert row to JSON object: %w", err)
		}
		if rowJson, err = json.Marshal(row); err != nil {
			return nil, fmt.Errorf("failed row to JSON: %w", err)
		}
		results = append(results, string(rowJson))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating %s table rows: %w", tableName, err)
	}

	return results, nil
}

// toJsonObj converts a row from the database to a JSON object
// It assumes that a next row is available!
func toJsonObj(nextRow *sql.Rows, columns []string) (map[string]interface{}, error) {
	// Create values and associate ptrs to them for row scanning
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	err := nextRow.Scan(valuePtrs...)
	if err != nil {
		return nil, fmt.Errorf("failed to scan row: %w", err)
	}
	result := make(map[string]interface{})
	for i, col := range columns {
		if values[i] == nil {
			continue
		}
		switch v := values[i].(type) {
		case []byte:
			jsonObj := make(map[string]interface{})
			if err = json.Unmarshal(v, &jsonObj); err == nil {
				values[i] = jsonObj
				break
			}
			jsonArr := make([]interface{}, 0)
			if err = json.Unmarshal(v, &jsonArr); err == nil {
				values[i] = jsonArr
				break
			}
			values[i] = string(v)
		default:
			values[i] = v
		}
		result[col] = values[i]
	}
	return result, nil
}
