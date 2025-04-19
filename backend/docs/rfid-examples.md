# RFID Integration Examples

This document provides practical examples of integrating with the MOTO RFID tracking system using various programming languages and frameworks.

## Setup

Before running these examples, you need to:

1. Register your device using the `POST /rfid/devices` endpoint
2. Store the API key securely
3. Replace `YOUR_API_KEY` and endpoint URLs in the examples with your actual values

## cURL Examples

### Register a new device

```bash
curl -X POST \
  "http://your-server/api/rfid/devices" \
  -H "Content-Type: application/json" \
  -d '{
    "device_id": "raspberry-pi-001",
    "name": "Main Entrance RFID",
    "description": "RFID reader at main entrance"
  }'
```

### Record a tag read

```bash
curl -X POST \
  "http://your-server/api/rfid/tag" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "tag_id": "abc123456",
    "reader_id": "raspberry-pi-001"
  }'
```

### Track student location

```bash
curl -X POST \
  "http://your-server/api/rfid/track-student" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "tag_id": "abc123456",
    "reader_id": "raspberry-pi-001",
    "location_type": "entry"
  }'
```

### Get room occupancy

```bash
curl -X GET \
  "http://your-server/api/rfid/room-occupancy?room_id=123" \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## Python Examples

### Basic setup with requests library

```python
import requests

API_BASE_URL = "http://your-server/api/rfid"
API_KEY = "YOUR_API_KEY"

headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

def record_tag_read(tag_id, reader_id):
    url = f"{API_BASE_URL}/tag"
    payload = {
        "tag_id": tag_id,
        "reader_id": reader_id
    }
    
    response = requests.post(url, json=payload, headers=headers)
    
    if response.status_code == 201:
        print(f"Tag read recorded: {response.json()}")
        return response.json()
    else:
        print(f"Error: {response.status_code}, {response.text}")
        return None

def track_student(tag_id, reader_id, location_type):
    url = f"{API_BASE_URL}/track-student"
    payload = {
        "tag_id": tag_id,
        "reader_id": reader_id,
        "location_type": location_type  # "entry", "wc", "schoolyard", or "exit"
    }
    
    response = requests.post(url, json=payload, headers=headers)
    
    if response.status_code == 200:
        print(f"Student tracked: {response.json()}")
        return response.json()
    else:
        print(f"Error: {response.status_code}, {response.text}")
        return None

def record_room_entry(tag_id, reader_id, room_id):
    url = f"{API_BASE_URL}/room-entry"
    payload = {
        "tag_id": tag_id,
        "reader_id": reader_id,
        "room_id": room_id
    }
    
    response = requests.post(url, json=payload, headers=headers)
    
    if response.status_code == 200:
        print(f"Room entry recorded: {response.json()}")
        return response.json()
    else:
        print(f"Error: {response.status_code}, {response.text}")
        return None

def get_room_occupancy(room_id=None):
    url = f"{API_BASE_URL}/room-occupancy"
    params = {}
    
    if room_id:
        params["room_id"] = room_id
    
    response = requests.get(url, params=params, headers=headers)
    
    if response.status_code == 200:
        print(f"Room occupancy: {response.json()}")
        return response.json()
    else:
        print(f"Error: {response.status_code}, {response.text}")
        return None
```

### RFID Tag Reader Integration (Raspberry Pi)

```python
#!/usr/bin/env python3
import RPi.GPIO as GPIO
import MFRC522
import requests
import signal
import time

# API Configuration
API_BASE_URL = "http://your-server/api/rfid"
API_KEY = "YOUR_API_KEY"
READER_ID = "raspberry-pi-001"

# Headers for API requests
headers = {
    "Authorization": f"Bearer {API_KEY}",
    "Content-Type": "application/json"
}

# Initialize RFID reader
continue_reading = True
MIFAREReader = MFRC522.MFRC522()

# Function to stop the program gracefully
def end_read(signal, frame):
    global continue_reading
    print("Stopping RFID reader...")
    continue_reading = False
    GPIO.cleanup()

# Hook the SIGINT
signal.signal(signal.SIGINT, end_read)

def record_tag_read(tag_id):
    url = f"{API_BASE_URL}/tag"
    payload = {
        "tag_id": tag_id,
        "reader_id": READER_ID
    }
    
    try:
        response = requests.post(url, json=payload, headers=headers)
        if response.status_code == 201:
            print(f"Tag read recorded: {tag_id}")
            return True
        else:
            print(f"Error recording tag: {response.status_code}, {response.text}")
            return False
    except Exception as e:
        print(f"Error connecting to API: {str(e)}")
        return False

def track_student_location(tag_id, location_type="entry"):
    url = f"{API_BASE_URL}/track-student"
    payload = {
        "tag_id": tag_id,
        "reader_id": READER_ID,
        "location_type": location_type
    }
    
    try:
        response = requests.post(url, json=payload, headers=headers)
        if response.status_code == 200:
            data = response.json()
            print(f"Student tracked: {data.get('name')} - {data.get('location')}")
            return True
        else:
            print(f"Error tracking student: {response.status_code}, {response.text}")
            return False
    except Exception as e:
        print(f"Error connecting to API: {str(e)}")
        return False

# Main loop to read RFID tags
print("Starting RFID reader. Press Ctrl+C to stop.")
last_read_id = None
last_read_time = 0

while continue_reading:
    # Scan for cards
    (status, TagType) = MIFAREReader.MFRC522_Request(MIFAREReader.PICC_REQIDL)

    # If a card is found
    if status == MIFAREReader.MI_OK:
        print("Card detected")
        
        # Get the UID of the card
        (status, uid) = MIFAREReader.MFRC522_Anticoll()
        
        if status == MIFAREReader.MI_OK:
            # Format UID as string
            card_id = '-'.join([str(x) for x in uid])
            
            # Simple debounce - don't record the same card twice in quick succession
            current_time = time.time()
            if card_id != last_read_id or (current_time - last_read_time) > 5:
                print(f"Card UID: {card_id}")
                
                # Record the tag read
                record_tag_read(card_id)
                
                # Track student location
                track_student_location(card_id)
                
                # Update last read
                last_read_id = card_id
                last_read_time = current_time
            
            # Slight delay before next read
            time.sleep(0.5)
```

## JavaScript (Node.js) Example

```javascript
const axios = require('axios');

const API_BASE_URL = 'http://your-server/api/rfid';
const API_KEY = 'YOUR_API_KEY';

const headers = {
  'Authorization': `Bearer ${API_KEY}`,
  'Content-Type': 'application/json'
};

async function recordTagRead(tagId, readerId) {
  try {
    const response = await axios.post(`${API_BASE_URL}/tag`, {
      tag_id: tagId,
      reader_id: readerId
    }, { headers });
    
    console.log('Tag read recorded:', response.data);
    return response.data;
  } catch (error) {
    console.error('Error recording tag read:', error.response?.data || error.message);
    return null;
  }
}

async function trackStudent(tagId, readerId, locationType) {
  try {
    const response = await axios.post(`${API_BASE_URL}/track-student`, {
      tag_id: tagId,
      reader_id: readerId,
      location_type: locationType // "entry", "wc", "schoolyard", or "exit"
    }, { headers });
    
    console.log('Student tracked:', response.data);
    return response.data;
  } catch (error) {
    console.error('Error tracking student:', error.response?.data || error.message);
    return null;
  }
}

async function recordRoomEntry(tagId, readerId, roomId) {
  try {
    const response = await axios.post(`${API_BASE_URL}/room-entry`, {
      tag_id: tagId,
      reader_id: readerId,
      room_id: roomId
    }, { headers });
    
    console.log('Room entry recorded:', response.data);
    return response.data;
  } catch (error) {
    console.error('Error recording room entry:', error.response?.data || error.message);
    return null;
  }
}

async function recordRoomExit(tagId, readerId, roomId) {
  try {
    const response = await axios.post(`${API_BASE_URL}/room-exit`, {
      tag_id: tagId,
      reader_id: readerId,
      room_id: roomId
    }, { headers });
    
    console.log('Room exit recorded:', response.data);
    return response.data;
  } catch (error) {
    console.error('Error recording room exit:', error.response?.data || error.message);
    return null;
  }
}

async function getRoomOccupancy(roomId = null) {
  try {
    const url = `${API_BASE_URL}/room-occupancy`;
    const params = roomId ? { room_id: roomId } : {};
    
    const response = await axios.get(url, { 
      headers,
      params
    });
    
    console.log('Room occupancy:', response.data);
    return response.data;
  } catch (error) {
    console.error('Error getting room occupancy:', error.response?.data || error.message);
    return null;
  }
}

// Example usage
async function main() {
  // Record a tag read
  await recordTagRead('abc123456', 'node-device-001');
  
  // Track a student location
  await trackStudent('abc123456', 'node-device-001', 'entry');
  
  // Record a student entering a room
  await recordRoomEntry('abc123456', 'node-device-001', 123);
  
  // Get room occupancy
  await getRoomOccupancy(123);
  
  // Record a student exiting a room
  await recordRoomExit('abc123456', 'node-device-001', 123);
}

main().catch(console.error);
```

## Tauri App Sync Example (Rust + Tauri)

### Rust code for Tauri app backend

```rust
use serde::{Deserialize, Serialize};
use std::sync::Mutex;
use std::time::{SystemTime, UNIX_EPOCH};
use std::collections::VecDeque;
use tauri::{command, State};
use reqwest::header::{HeaderMap, HeaderValue, AUTHORIZATION, CONTENT_TYPE};

// Data structures
#[derive(Debug, Serialize, Deserialize, Clone)]
struct TagRead {
    tag_id: String,
    reader_id: String,
    timestamp: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct SyncRequest {
    device_id: String,
    app_version: String,
    data: Vec<TagRead>,
}

#[derive(Debug, Serialize, Deserialize)]
struct ApiResponse {
    success: bool,
    message: String,
}

// Application state
struct AppState {
    tag_queue: Mutex<VecDeque<TagRead>>,
    api_key: String,
    device_id: String,
}

// Initialize app state
fn init_app_state() -> AppState {
    AppState {
        tag_queue: Mutex::new(VecDeque::new()),
        api_key: "YOUR_API_KEY".to_string(),
        device_id: "tauri-app-001".to_string(),
    }
}

// Record a tag read and store in queue
#[command]
fn record_tag(state: State<AppState>, tag_id: String) -> Result<String, String> {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_err(|e| e.to_string())?;
    
    // Format timestamp in ISO format
    let dt = chrono::DateTime::<chrono::Utc>::from(SystemTime::now());
    let timestamp = dt.to_rfc3339();
    
    let tag_read = TagRead {
        tag_id,
        reader_id: state.device_id.clone(),
        timestamp,
    };
    
    // Add to queue
    let mut queue = state.tag_queue.lock().map_err(|e| e.to_string())?;
    queue.push_back(tag_read.clone());
    
    // Return success message
    Ok(format!("Tag {} recorded at {}", tag_read.tag_id, tag_read.timestamp))
}

// Sync data with server
#[command]
async fn sync_with_server(state: State<'_, AppState>) -> Result<String, String> {
    // Get tags from queue
    let mut queue = state.tag_queue.lock().map_err(|e| e.to_string())?;
    
    if queue.is_empty() {
        return Ok("No tags to sync".to_string());
    }
    
    // Prepare data to send
    let tags: Vec<TagRead> = queue.drain(..).collect();
    let sync_data = SyncRequest {
        device_id: state.device_id.clone(),
        app_version: env!("CARGO_PKG_VERSION").to_string(),
        data: tags.clone(),
    };
    
    // Prepare headers
    let mut headers = HeaderMap::new();
    headers.insert(
        AUTHORIZATION,
        HeaderValue::from_str(&format!("Bearer {}", state.api_key))
            .map_err(|e| e.to_string())?,
    );
    headers.insert(
        CONTENT_TYPE,
        HeaderValue::from_static("application/json"),
    );
    
    // Send request to server
    let client = reqwest::Client::new();
    let response = client.post("http://your-server/api/rfid/app/sync")
        .headers(headers)
        .json(&sync_data)
        .send()
        .await
        .map_err(|e| e.to_string())?;
    
    if response.status().is_success() {
        let api_response: ApiResponse = response.json()
            .await
            .map_err(|e| e.to_string())?;
        
        Ok(format!("Sync successful: {}", api_response.message))
    } else {
        let status = response.status();
        let error_text = response.text().await.map_err(|e| e.to_string())?;
        
        // If sync fails, add tags back to queue
        for tag in tags {
            queue.push_back(tag);
        }
        
        Err(format!("Sync failed: {} - {}", status, error_text))
    }
}

// Check server status
#[command]
async fn check_server_status(state: State<'_, AppState>) -> Result<String, String> {
    // Prepare headers
    let mut headers = HeaderMap::new();
    headers.insert(
        AUTHORIZATION,
        HeaderValue::from_str(&format!("Bearer {}", state.api_key))
            .map_err(|e| e.to_string())?,
    );
    
    // Send request to server
    let client = reqwest::Client::new();
    let response = client.get("http://your-server/api/rfid/app/status")
        .headers(headers)
        .send()
        .await
        .map_err(|e| e.to_string())?;
    
    if response.status().is_success() {
        let status_text = response.text().await.map_err(|e| e.to_string())?;
        Ok(status_text)
    } else {
        let status = response.status();
        let error_text = response.text().await.map_err(|e| e.to_string())?;
        Err(format!("Status check failed: {} - {}", status, error_text))
    }
}

// Tauri application setup
fn main() {
    let app_state = init_app_state();
    
    tauri::Builder::default()
        .manage(app_state)
        .invoke_handler(tauri::generate_handler![
            record_tag,
            sync_with_server,
            check_server_status
        ])
        .run(tauri::generate_context!())
        .expect("Error while running Tauri application");
}
```

## Java Example

```java
import java.io.IOException;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;
import org.json.JSONObject;

public class RFIDClient {
    private final String apiBaseUrl;
    private final String apiKey;
    private final HttpClient httpClient;
    
    public RFIDClient(String apiBaseUrl, String apiKey) {
        this.apiBaseUrl = apiBaseUrl;
        this.apiKey = apiKey;
        this.httpClient = HttpClient.newBuilder()
                .connectTimeout(Duration.ofSeconds(10))
                .build();
    }
    
    public JSONObject recordTagRead(String tagId, String readerId) throws IOException, InterruptedException {
        String url = apiBaseUrl + "/tag";
        
        JSONObject requestBody = new JSONObject();
        requestBody.put("tag_id", tagId);
        requestBody.put("reader_id", readerId);
        
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .header("Authorization", "Bearer " + apiKey)
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(requestBody.toString()))
                .build();
        
        HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
        
        if (response.statusCode() == 201) {
            return new JSONObject(response.body());
        } else {
            throw new IOException("Error recording tag read: " + response.statusCode() + " " + response.body());
        }
    }
    
    public JSONObject trackStudent(String tagId, String readerId, String locationType) 
            throws IOException, InterruptedException {
        String url = apiBaseUrl + "/track-student";
        
        JSONObject requestBody = new JSONObject();
        requestBody.put("tag_id", tagId);
        requestBody.put("reader_id", readerId);
        requestBody.put("location_type", locationType);
        
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .header("Authorization", "Bearer " + apiKey)
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(requestBody.toString()))
                .build();
        
        HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
        
        if (response.statusCode() == 200) {
            return new JSONObject(response.body());
        } else {
            throw new IOException("Error tracking student: " + response.statusCode() + " " + response.body());
        }
    }
    
    public JSONObject recordRoomEntry(String tagId, String readerId, long roomId) 
            throws IOException, InterruptedException {
        String url = apiBaseUrl + "/room-entry";
        
        JSONObject requestBody = new JSONObject();
        requestBody.put("tag_id", tagId);
        requestBody.put("reader_id", readerId);
        requestBody.put("room_id", roomId);
        
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .header("Authorization", "Bearer " + apiKey)
                .header("Content-Type", "application/json")
                .POST(HttpRequest.BodyPublishers.ofString(requestBody.toString()))
                .build();
        
        HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
        
        if (response.statusCode() == 200) {
            return new JSONObject(response.body());
        } else {
            throw new IOException("Error recording room entry: " + response.statusCode() + " " + response.body());
        }
    }
    
    public JSONObject getRoomOccupancy(Long roomId) throws IOException, InterruptedException {
        String url = apiBaseUrl + "/room-occupancy";
        
        if (roomId != null) {
            url += "?room_id=" + roomId;
        }
        
        HttpRequest request = HttpRequest.newBuilder()
                .uri(URI.create(url))
                .header("Authorization", "Bearer " + apiKey)
                .GET()
                .build();
        
        HttpResponse<String> response = httpClient.send(request, HttpResponse.BodyHandlers.ofString());
        
        if (response.statusCode() == 200) {
            return new JSONObject(response.body());
        } else {
            throw new IOException("Error getting room occupancy: " + response.statusCode() + " " + response.body());
        }
    }
    
    public static void main(String[] args) {
        RFIDClient client = new RFIDClient("http://your-server/api/rfid", "YOUR_API_KEY");
        
        try {
            // Record a tag read
            JSONObject tagReadResult = client.recordTagRead("abc123456", "java-client-001");
            System.out.println("Tag read recorded: " + tagReadResult.toString(2));
            
            // Track student location
            JSONObject trackingResult = client.trackStudent("abc123456", "java-client-001", "entry");
            System.out.println("Student tracked: " + trackingResult.toString(2));
            
            // Record room entry
            JSONObject roomEntryResult = client.recordRoomEntry("abc123456", "java-client-001", 123);
            System.out.println("Room entry recorded: " + roomEntryResult.toString(2));
            
            // Get room occupancy
            JSONObject occupancyResult = client.getRoomOccupancy(123L);
            System.out.println("Room occupancy: " + occupancyResult.toString(2));
            
        } catch (Exception e) {
            e.printStackTrace();
        }
    }
}
```

## Integration Best Practices

1. **Error Handling**
   - Always check status codes and handle errors gracefully
   - Implement retry logic for network failures
   - Log both successful and failed API calls

2. **Caching and Offline Operation**
   - Queue transactions when offline and sync when connection is restored
   - Implement local storage for offline operation
   - Use a background service for synchronization

3. **Security**
   - Store API keys securely (environment variables, secure storage)
   - Never expose API keys in client-side code
   - Validate all input before sending to the API

4. **Performance**
   - Batch tag reads when possible to reduce API calls
   - Implement request throttling to stay within rate limits
   - Use connection pooling in high-throughput scenarios

5. **Monitoring**
   - Log API requests and responses for debugging
   - Monitor success rates and response times
   - Set up alerts for unusual patterns or failures