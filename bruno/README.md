# Project Phoenix Bruno API Tests

Simplified API testing suite for Project Phoenix using [Bruno](https://usebruno.com/). Designed for daily development workflow - quick, simple, and reliable.

## 🚀 Quick Start

### Prerequisites
- Bruno CLI: `brew install bruno-cli` or `npm install -g @usebruno/cli`
- jq: `brew install jq` (for token handling)
- Backend running: `docker compose up -d`

### Running Tests

#### One Simple Runner
```bash
./dev-test.sh groups    # Test groups API (25 groups) - 44ms
./dev-test.sh students  # Test students API (50 students) - 50ms
./dev-test.sh rooms     # Test rooms API (24 rooms) - 19ms
./dev-test.sh devices   # Test RFID device auth - 117ms
./dev-test.sh all       # Test everything - 252ms (auto-runs cleanup first)
./dev-test.sh examples  # View API examples
./dev-test.sh manual    # Pre-release checks
```

**How it works:** Each command gets a fresh admin token and tests the API.

**Cleanup:** Running `./dev-test.sh all` automatically checks out all active students first to ensure a clean test state.

#### Bruno GUI (Optional)
1. Open Bruno app → Open Collection → Select this directory
2. Select "Local" environment → Run requests manually

## 🔧 Setup for New Team Members

### First Time Setup

1. **Install Bruno CLI**:
   ```bash
   brew install bruno-cli
   # or
   npm install -g @usebruno/cli
   ```

2. **Install jq** (for token handling):
   ```bash
   brew install jq
   ```

3. **Configure Environment**:
   ```bash
   cd bruno
   cp environments/Local.bru.example environments/Local.bru
   ```

4. **Edit Local.bru** with your credentials:
   ```bash
   # Open bruno/environments/Local.bru and update:

   deviceApiKey: <get from iot.devices table or ask admin>
   staffPIN: <your 4-digit device PIN>
   staffID: <your staff ID from database>
   testStaffEmail: <your staff email>
   testStaffPassword: <your password>

   # Test student data (update if seed data changes)
   testStudent1RFID: <RFID tag for first test student>
   testStudent1Name: <Full name of first test student>
   testStudent2RFID: <RFID tag for second test student>
   testStudent2Name: <Full name of second test student>
   testStudent3RFID: <RFID tag for third test student>
   testStudent3Name: <Full name of third test student>
   ```

5. **Verify Setup**:
   ```bash
   ./dev-test.sh groups    # Should return 200 OK
   ```

### Getting Credentials

- **Device API Key**: Ask admin or query database:
  ```sql
  SELECT api_key FROM iot.devices WHERE name = 'Development Device';
  ```
- **Staff PIN**: Set via API or use default from seed data (1234)
- **Staff ID**: Query database or use your existing account
- **Email/Password**: Your staff account credentials
- **Test Student Data**: Query database for test students (RFIDs and names):
  ```sql
  SELECT p.first_name, p.last_name, p.tag_id,
         CONCAT(p.first_name, ' ', p.last_name) as full_name
  FROM users.persons p
  JOIN users.students s ON s.person_id = p.id
  WHERE p.tag_id IS NOT NULL
  ORDER BY s.id
  LIMIT 3;
  ```
  Use the first 3 students for testStudent1, testStudent2, testStudent3

### Manual Cleanup

If you need to manually checkout all students before running tests:
```bash
./cleanup-before-tests.sh
```

This is automatically run when using `./dev-test.sh all`.

### Security Notes

⚠️ **NEVER commit Local.bru** - it contains your personal credentials!

- Local.bru is gitignored automatically
- Only Local.bru.example should be committed
- Each team member has their own Local.bru with their credentials

## 📁 Simplified Structure

```
bruno/
├── dev/                   # Daily development workflow
│   ├── auth.bru          # Admin login
│   ├── groups.bru        # Groups API test
│   ├── students.bru      # Students API test
│   ├── rooms.bru         # Rooms API test
│   └── device-auth.bru   # RFID device test
├── examples/             # API documentation
│   ├── auth-example.bru  # How to authenticate
│   └── device-example.bru # How to use device auth
├── manual/               # Pre-release checks
│   └── critical-flows.bru # Key user journeys
├── dev-test.sh           # Simple test runner
└── environments/
    └── Local.bru         # Environment config
```

## 🔐 Authentication

The API uses JWT tokens for authentication:
- **Access Token**: Valid for 15 minutes
- **Refresh Token**: Valid for 1 hour

### Test Accounts
```
Admin: admin@example.com / Test1234%    (for general API testing)
Test Teacher: y.wenger@gmx.de / Test1234%  (for device authentication, PIN: 1234)
```

### Account Usage
- **Admin Account**: Used automatically by `dev-test.sh` for all API tests
- **Teacher Account**: Used for device authentication (`y.wenger@gmx.de`, PIN: 1234)

### How It Works
1. `dev-test.sh` gets fresh admin token via curl
2. Passes token to Bruno via `--env-var accessToken="$TOKEN"`
3. No token persistence issues, always works

## 🧑‍💻 Development Workflows

### Frontend Developer
```bash
# Building groups page?
./dev-test.sh groups    # ✅ 25 groups available

# Building student list? 
./dev-test.sh students  # ✅ 50 students available

# Building room view?
./dev-test.sh rooms     # ✅ 24 rooms available
```

### Backend Developer  
```bash
# Made changes to groups API?
go test ./api/groups    # Backend tests
./dev-test.sh groups    # Frontend integration check

# Added new endpoint?
go test ./...           # All backend tests  
./dev-test.sh all       # Frontend integration check
```

### Pre-Deployment
```bash
./dev-test.sh all       # Everything working? → Deploy
```

## ✅ What's Tested

**Core APIs (all working):**
- **Authentication**: Admin login automatic
- **Groups API**: 25 groups returned (42ms)
- **Students API**: 50 students returned (50ms)
- **Rooms API**: 24 rooms returned (19ms)  
- **Device Auth**: RFID authentication (94ms)

**Complete test suite**: 9 files, ~270ms total

## 🎯 Purpose

This isn't comprehensive testing - that's what `go test ./...` does. 

This is **development confidence** - quick checks that APIs work for frontend development.

## 🔧 Troubleshooting

**If tests fail:**
```bash
# Check backend is running
docker compose ps

# Check if you have jq installed  
jq --version

# Test login manually
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"Test1234%"}'
```

**Most common issue:** Backend not running → `docker compose up -d`

---

**Clean Structure:** The old complex structure has been removed. Only the simplified structure (dev/, examples/, manual/) remains - 9 test files total.