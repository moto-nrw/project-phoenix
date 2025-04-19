# RFID Integration Guide

This guide provides detailed instructions for integrating with the MOTO RFID tracking system.

## Architecture Overview

The MOTO RFID tracking system consists of several components:

1. **RFID Readers**: Physical devices that read RFID tags
2. **Tauri Desktop App**: Desktop application that can sync RFID data
3. **Backend Server**: Central server that processes and stores RFID data
4. **Database**: Stores student, tag, and location data

## Authentication

Most RFID endpoints require authentication using an API key. These keys are device-specific and should be kept secure.

### Getting an API Key

Before using the RFID API, you need to register your device:

1. Send a POST request to `/api/rfid/devices` with your device information
2. Store the returned API key securely - it's only shown once!

```json
// Request
POST /api/rfid/devices
{
  "device_id": "unique-device-id",
  "name": "RFID Reader 1",
  "description": "RFID reader at main entrance"
}

// Response
{
  "success": true,
  "message": "Device registered successfully",
  "device": {
    "id": 1,
    "device_id": "unique-device-id",
    "name": "RFID Reader 1",
    "description": "RFID reader at main entrance",
    "status": "active",
    "created_at": "2023-10-15T14:30:00Z",
    "updated_at": "2023-10-15T14:30:00Z"
  },
  "api_key": "YOUR_API_KEY",
  "device_id": "unique-device-id"
}
```

### Using API Keys

Include the API key in all requests to protected endpoints:

```
Authorization: Bearer YOUR_API_KEY
```

## Security Best Practices

1. **API Key Protection**
   - Store API keys securely, using environment variables or secure storage
   - Never hardcode keys in your application
   - Rotate keys periodically or if compromised

2. **Transport Security**
   - Always use HTTPS for all API communications
   - Validate server certificates

3. **Device Management**
   - Limit permissions for devices to only what they need
   - Deactivate devices that are no longer in use

4. **Data Protection**
   - Minimize storage of sensitive data on client devices
   - Encrypt any locally stored data
   - Regularly sync and clear local data

## Handling Errors

All API endpoints return consistent error responses:

```json
{
  "status": "Invalid request.",
  "code": 0,
  "error": "Detailed error message"
}
```

Common HTTP status codes:
- `200`: Success
- `201`: Created (for POST requests that create resources)
- `400`: Bad Request (client error)
- `401`: Unauthorized (missing or invalid API key)
- `404`: Not Found
- `422`: Unprocessable Entity (malformed request)
- `500`: Internal Server Error

## Rate Limiting

The API implements rate limiting to prevent abuse:

- Standard endpoints: 10 requests per second per device
- Data-intensive endpoints: 5 requests per second per device

If you exceed these limits, you'll receive a `429 Too Many Requests` response.

## Implementation Guidelines

### RFID Reader Integration

For hardware RFID readers (like Raspberry Pi with RFID modules):

1. Register your device and obtain an API key
2. Configure the reader to send scanned tag data to the API
3. Use the `/rfid/tag` endpoint to record tag reads
4. Implement proper error handling and retry logic

### Tauri Desktop App Integration

For Tauri desktop applications:

1. Register your app as a device and obtain an API key
2. Implement local storage for tag reads when offline
3. Sync data periodically using the `/rfid/app/sync` endpoint
4. Check system status using the `/rfid/app/status` endpoint

### Web Application Integration

For web applications:

1. Register your application as a device
2. Use client-side JavaScript for real-time updates
3. Implement proper authentication flows
4. Use WebSockets for real-time updates if available

## Testing

We provide a sandbox environment for testing your integration:

- Sandbox URL: `https://sandbox.moto-system.example.com/api`
- Test API key: `sandbox_test_key`

To verify your integration is working:

1. Register a test device
2. Send test tag reads
3. Verify the data appears in the sandbox dashboard

## Common Issues and Solutions

### Authentication Issues

**Problem**: API requests are returning 401 Unauthorized
**Solution**: 
- Verify your API key is correct
- Ensure it's being sent with the `Bearer` prefix
- Check if the device is still active

### Data Sync Problems

**Problem**: Data isn't appearing in the system after sync
**Solution**:
- Check network connectivity
- Verify the sync response for errors
- Ensure your device ID matches the registered device

### Rate Limiting

**Problem**: Receiving 429 Too Many Requests errors
**Solution**:
- Implement exponential backoff
- Batch requests where possible
- Optimize your code to reduce unnecessary API calls

## Support

If you encounter any issues with the RFID integration:

- Check the [detailed API documentation](https://docs.moto-system.example.com/api)
- Contact support at support@moto-system.example.com
- Join our developer community at [community.moto-system.example.com](https://community.moto-system.example.com)

## API Versioning

The current API version is v1. All endpoints are prefixed with `/api/rfid`.

When we release new API versions, we'll maintain backward compatibility and provide migration guides.