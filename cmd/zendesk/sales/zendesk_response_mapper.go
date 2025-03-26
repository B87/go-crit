package zendesk_sales

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/viper"
)

// ZendeskResponseMapper specifically maps Zendesk API responses
type ZendeskResponseMapper struct{}

// MapResponse parses the Zendesk API response and extracts contacts
func (m *ZendeskResponseMapper) MapResponse(responseBody []byte) ([]ZendeskContactEntity, int64, error) {
	// Try different known Zendesk response formats in order of likelihood
	parsers := []responseParser{
		parseStandardFormat,
		parseSimpleDataFormat,
		parseItemsFormat,
	}

	// Try each parser in sequence
	for _, parser := range parsers {
		contacts, total, err := parser(responseBody)
		if err == nil && len(contacts) > 0 {
			if viper.GetBool("verbose") {
				fmt.Printf("Successfully parsed %d contacts\n", len(contacts))
			}
			return contacts, total, nil
		}
	}

	// If debug is enabled, print raw response structure
	logRawResponseStructure(responseBody)

	// If we get here, we couldn't parse the response
	return nil, 0, fmt.Errorf("no contacts found in response or unsupported format")
}

// responseParser is a function type for parsing different response formats
type responseParser func([]byte) ([]ZendeskContactEntity, int64, error)

// parseStandardFormat parses the response in the standard Zendesk format with items array
func parseStandardFormat(responseBody []byte) ([]ZendeskContactEntity, int64, error) {
	// Standard Zendesk API format with items containing data/meta
	var response struct {
		Items []struct {
			Data ZendeskContactEntity `json:"data"`
			Meta struct {
				Version int    `json:"version"`
				Type    string `json:"type"`
			} `json:"meta"`
		} `json:"items"`
		Meta struct {
			Type  string `json:"type"`
			Count int64  `json:"count"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, 0, err
	}

	if len(response.Items) == 0 {
		return nil, 0, fmt.Errorf("no items found in standard format")
	}

	// Extract contacts from items
	contacts := make([]ZendeskContactEntity, 0, len(response.Items))
	for _, item := range response.Items {
		contacts = append(contacts, item.Data)
	}

	return contacts, response.Meta.Count, nil
}

// parseSimpleDataFormat parses the response in the simple format with data array
func parseSimpleDataFormat(responseBody []byte) ([]ZendeskContactEntity, int64, error) {
	// Format with direct data array
	var response struct {
		Data []ZendeskContactEntity `json:"data"`
		Meta struct {
			Total int64 `json:"total"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, 0, err
	}

	if len(response.Data) == 0 {
		return nil, 0, fmt.Errorf("no data found in simple format")
	}

	return response.Data, response.Meta.Total, nil
}

// parseItemsFormat parses the response with items array directly containing contacts
func parseItemsFormat(responseBody []byte) ([]ZendeskContactEntity, int64, error) {
	// Format with items directly containing contacts
	var response struct {
		Items []ZendeskContactEntity `json:"items"`
		Meta  struct {
			Type  string `json:"type"`
			Count int64  `json:"count"`
		} `json:"meta"`
	}

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, 0, err
	}

	if len(response.Items) == 0 {
		return nil, 0, fmt.Errorf("no items found in direct format")
	}

	return response.Items, response.Meta.Count, nil
}

// logRawResponseStructure logs the raw structure of the response for debugging
func logRawResponseStructure(responseBody []byte) {
	if viper.GetBool("verbose") {
		var rawResponse map[string]interface{}
		if err := json.Unmarshal(responseBody, &rawResponse); err == nil {
			fmt.Println("Raw response structure:")
			for key, value := range rawResponse {
				fmt.Printf("Key: %s, Type: %T\n", key, value)
			}
		} else {
			fmt.Printf("Could not parse response as JSON: %v\n", err)
		}
	}
}
