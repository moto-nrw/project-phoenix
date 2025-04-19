# RFID API Authentication Examples

This document provides examples of how to authenticate with the MOTO RFID API using API keys.

## Getting an API Key

Before you can use most RFID endpoints, you need to register your device and obtain an API key.

```bash
# Register a new device
curl -X POST \
  "http://localhost:3000/api/rfid/devices" \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "my-unique-device-id",
    "name": "My RFID Reader",
    "description": "RFID reader at school entrance"
  }'
```

Response:
```json
{
  "success": true,
  "message": "Device registered successfully",
  "device": {
    "id": 1,
    "device_id": "my-unique-device-id",
    "name": "My RFID Reader",
    "description": "RFID reader at school entrance",
    "status": "active",
    "created_at": "2023-10-15T14:30:00Z",
    "updated_at": "2023-10-15T14:30:00Z"
  },
  "api_key": "YOUR_API_KEY",
  "device_id": "my-unique-device-id"
}
```

**IMPORTANT**: Store this API key securely. It will only be shown once during registration.

## Using the API Key

All protected RFID endpoints require the API key to be included in the `Authorization` header with the `Bearer` prefix:

```bash
# Record a tag read
curl -X POST \
  "http://localhost:3000/api/rfid/tag" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "tag_id": "abc123456",
    "reader_id": "my-unique-device-id"
  }'
```

## Authenticating GET Requests

For GET requests, you can also include the API key as a query parameter:

```bash
# Get all tags
curl -X GET \
  "http://localhost:3000/api/rfid/tags?api_key=YOUR_API_KEY"
```

## API Key Authentication Flow

The authentication flow works as follows:

1. The API extracts the API key from the `Authorization` header or query parameter
2. The system looks up the device associated with this API key
3. It verifies that the device is active
4. If validation is successful, the request is processed
5. The device's last IP address is updated automatically

## Error Responses

If authentication fails, you will receive a 401 Unauthorized response:

```json
{
  "status": "Unauthorized.",
  "error": "API key required"
}
```

or

```json
{
  "status": "Unauthorized.",
  "error": "invalid API key"
}
```

or 

```json
{
  "status": "Unauthorized.",
  "error": "device is not active"
}
```

## API Key Management Best Practices

1. **Store Securely**: Never hardcode API keys in applications or scripts
2. **Use Environment Variables**: Store keys as environment variables or in secure storage
3. **Rotate Regularly**: Periodically regenerate API keys (requires creating a new device and updating your application)
4. **Limit Access**: Only grant access to the specific endpoints needed
5. **Monitor Usage**: Regularly check device sync history for unusual patterns
6. **Deactivate Unused Keys**: Set device status to "inactive" when no longer needed

## Code Examples

### Python Example

```python
import requests
import os

# Get API key from environment variable
API_KEY = os.environ.get("RFID_API_KEY")
API_BASE_URL = "http://localhost:3000/api/rfid"

# Set up headers with API key
headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

def record_tag_read(tag_id, reader_id):
    """
    Record an RFID tag read.
    """
    payload = {
        "tag_id": tag_id,
        "reader_id": reader_id
    }
    
    response = requests.post(
        f"{API_BASE_URL}/tag",
        headers=headers,
        json=payload
    )
    
    if response.status_code == 201:
        print(f"Successfully recorded tag: {tag_id}")
        return response.json()
    else:
        print(f"Error: {response.status_code}, {response.text}")
        return None

def get_all_tags():
    """
    Get all recorded RFID tags.
    """
    response = requests.get(
        f"{API_BASE_URL}/tags",
        headers=headers
    )
    
    if response.status_code == 200:
        return response.json()
    else:
        print(f"Error: {response.status_code}, {response.text}")
        return None
```

### JavaScript Example

```javascript
const axios = require('axios');

// API configuration
const apiKey = process.env.RFID_API_KEY;
const baseUrl = 'http://localhost:3000/api/rfid';

// Create axios instance with default headers
const apiClient = axios.create({
  baseURL: baseUrl,
  headers: {
    'Authorization': `Bearer ${apiKey}`,
    'Content-Type': 'application/json'
  }
});

// Record a tag read
async function recordTagRead(tagId, readerId) {
  try {
    const response = await apiClient.post('/tag', {
      tag_id: tagId,
      reader_id: readerId
    });
    
    console.log('Tag recorded:', response.data);
    return response.data;
  } catch (error) {
    console.error('Error recording tag:', error.response?.data || error.message);
    throw error;
  }
}

// Get room occupancy
async function getRoomOccupancy(roomId) {
  try {
    const url = roomId ? `/room-occupancy?room_id=${roomId}` : '/room-occupancy';
    const response = await apiClient.get(url);
    
    console.log('Room occupancy:', response.data);
    return response.data;
  } catch (error) {
    console.error('Error getting room occupancy:', error.response?.data || error.message);
    throw error;
  }
}
```

### Go Example

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

const (
	apiBaseURL = "http://localhost:3000/api/rfid"
)

// TagReadRequest represents the payload for recording a tag read
type TagReadRequest struct {
	TagID    string `json:"tag_id"`
	ReaderID string `json:"reader_id"`
}

// Tag represents an RFID tag read response
type Tag struct {
	ID        int64  `json:"id"`
	TagID     string `json:"tag_id"`
	ReaderID  string `json:"reader_id"`
	CreatedAt string `json:"created_at"`
}

// RFID client for making authenticated requests
type RFIDClient struct {
	apiKey string
	client *http.Client
}

// NewRFIDClient creates a new RFID API client
func NewRFIDClient(apiKey string) *RFIDClient {
	return &RFIDClient{
		apiKey: apiKey,
		client: &http.Client{},
	}
}

// RecordTagRead sends a request to record a tag read
func (c *RFIDClient) RecordTagRead(tagID, readerID string) (*Tag, error) {
	reqData := TagReadRequest{
		TagID:    tagID,
		ReaderID: readerID,
	}
	
	reqBody, err := json.Marshal(reqData)
	if err != nil {
		return nil, err
	}
	
	req, err := http.NewRequest(http.MethodPost, apiBaseURL+"/tag", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	
	// Add API key authentication
	req.Header.Add("Authorization", "Bearer "+c.apiKey)
	req.Header.Add("Content-Type", "application/json")
	
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: %d, %s", resp.StatusCode, string(bodyBytes))
	}
	
	var tag Tag
	if err := json.NewDecoder(resp.Body).Decode(&tag); err != nil {
		return nil, err
	}
	
	return &tag, nil
}

func main() {
	// Get API key from environment
	apiKey := os.Getenv("RFID_API_KEY")
	if apiKey == "" {
		fmt.Println("RFID_API_KEY environment variable not set")
		os.Exit(1)
	}
	
	client := NewRFIDClient(apiKey)
	
	// Record a tag read
	tag, err := client.RecordTagRead("abc123456", "reader-001")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("Successfully recorded tag: %+v\n", tag)
}
```

## Troubleshooting

### Authentication Issues

- **401 Unauthorized**: Check that your API key is correct and that it's being sent with the `Bearer` prefix
- **Device Inactive**: Ensure your device has "active" status in the system

### API Key Security

If you believe your API key has been compromised:

1. Create a new device with the same ID and get a new API key
2. Update your application to use the new API key
3. Set the old device's status to "inactive"