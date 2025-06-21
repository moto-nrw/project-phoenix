# Token Family Security Implementation

## Overview

The JWT token refresh implementation in Project Phoenix uses a sophisticated **Token Family** tracking system to prevent token theft and replay attacks. This document describes the security architecture and implementation details.

## Key Security Features

### 1. Token Family Tracking

Each refresh token belongs to a "family" identified by:
- **family_id**: A unique UUID identifying the token lineage
- **generation**: An incrementing counter tracking the token's position in the family tree

When a token is refreshed:
1. The old token is deleted
2. A new token is created with the same `family_id` but incremented `generation`
3. Only the latest generation token in a family is valid

### 2. Token Theft Detection

The system detects potential token theft by checking if an older generation token is used after a newer one exists:

```go
if dbToken.Generation < latestToken.Generation {
    // Token theft detected - entire family is invalidated
    tokenRepo.DeleteByFamilyID(ctx, dbToken.FamilyID)
    return error("Token theft detected")
}
```

This prevents attackers from using stolen tokens even if they obtain them.

### 3. Single Session Enforcement

On login, all existing tokens for the account are deleted:
```go
tokenRepo.DeleteByAccountID(ctx, account.ID)
```

This ensures only one active session per account, preventing token accumulation and reducing attack surface.

## API Endpoints

### POST /auth/refresh

Refreshes an expired access token using a valid refresh token.

**Request Headers:**
```
Authorization: Bearer <current_access_token>
```

**Request Body:**
```json
{
  "refresh_token": "string"
}
```

**Success Response (200 OK):**
```json
{
  "access_token": "eyJhbGciOiJIUzI1NiIs...",
  "refresh_token": "new-refresh-token-uuid",
  "expires_in": 900,
  "refresh_expires_in": 86400
}
```

**Error Responses:**

- **401 Unauthorized**: Token expired, invalid, or theft detected
```json
{
  "status": "error",
  "message": "Invalid or expired token",
  "code": "TOKEN_INVALID"
}
```

- **403 Forbidden**: Account disabled
```json
{
  "status": "error",
  "message": "Account is disabled",
  "code": "ACCOUNT_DISABLED"
}
```

## Token Lifecycle

1. **Login**: Creates new token family (generation 1)
2. **Refresh**: Increments generation, deletes old token
3. **Logout**: Deletes all tokens for the account
4. **Theft Detection**: Invalidates entire token family

## Database Schema

### auth.tokens Table

```sql
CREATE TABLE auth.tokens (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
    token TEXT NOT NULL,
    refresh_token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    refresh_expires_at TIMESTAMP NOT NULL,
    family_id VARCHAR(36),
    generation BIGINT DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tokens_family ON auth.tokens(family_id);
CREATE INDEX idx_tokens_account ON auth.tokens(account_id);
```

## Audit Logging

All authentication events are logged in `audit.auth_events`:

```sql
CREATE TABLE audit.auth_events (
    id BIGSERIAL PRIMARY KEY,
    account_id BIGINT NOT NULL,
    event_type VARCHAR(50) NOT NULL, -- 'login', 'logout', 'token_refresh', 'token_refresh_failed'
    success BOOLEAN NOT NULL,
    ip_address VARCHAR(45),
    user_agent TEXT,
    error_reason TEXT,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Security Best Practices

1. **Token Expiry**:
   - Access tokens: 15 minutes
   - Refresh tokens: 24 hours
   - Configurable via environment variables

2. **HTTPS Only**: Tokens should only be transmitted over HTTPS in production

3. **Secure Storage**: 
   - Tokens stored in HTTP-only cookies when possible
   - Never log full token values

4. **Rate Limiting**: Token refresh endpoints should be rate-limited

5. **Monitoring**: Watch for:
   - Multiple failed refresh attempts
   - Token theft detection events
   - Unusual refresh patterns

## Implementation Details

### Token Refresh Flow

```go
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (access, refresh string, err error) {
    // 1. Validate refresh token exists and not expired
    dbToken, err := s.tokenRepo.GetByRefreshToken(ctx, refreshToken)
    if err != nil || dbToken.RefreshExpiresAt.Before(time.Now()) {
        return "", "", ErrTokenExpired
    }

    // 2. Check for token theft
    if dbToken.FamilyID != "" {
        latest, _ := s.tokenRepo.GetLatestTokenInFamily(ctx, dbToken.FamilyID)
        if latest != nil && latest.Generation > dbToken.Generation {
            // Token theft detected
            s.tokenRepo.DeleteByFamilyID(ctx, dbToken.FamilyID)
            s.auditRepo.LogAuthEvent(ctx, &audit.AuthEvent{
                AccountID:   dbToken.AccountID,
                EventType:   "token_refresh_failed",
                Success:     false,
                ErrorReason: "Token theft detected - family invalidated",
            })
            return "", "", ErrInvalidToken
        }
    }

    // 3. Validate account is still active
    account, err := s.accountRepo.GetByID(ctx, dbToken.AccountID)
    if err != nil || !account.Enabled {
        return "", "", ErrAccountDisabled
    }

    // 4. Generate new tokens
    newAccess, newRefresh := s.generateTokens(account)
    
    // 5. Create new token record
    newToken := &auth.Token{
        AccountID:        account.ID,
        Token:           newAccess,
        RefreshToken:    newRefresh,
        FamilyID:        dbToken.FamilyID,
        Generation:      dbToken.Generation + 1,
        ExpiresAt:       time.Now().Add(s.accessExpiry),
        RefreshExpiresAt: time.Now().Add(s.refreshExpiry),
    }

    // 6. Delete old token and save new one (transaction)
    s.tokenRepo.Delete(ctx, dbToken.ID)
    s.tokenRepo.Create(ctx, newToken)

    // 7. Log success
    s.auditRepo.LogAuthEvent(ctx, &audit.AuthEvent{
        AccountID: account.ID,
        EventType: "token_refresh",
        Success:   true,
    })

    return newAccess, newRefresh, nil
}
```

## Frontend Integration

The frontend implements multiple layers of refresh protection:

1. **JWT Callback** in NextAuth prevents concurrent refreshes
2. **Token Refresh Manager** singleton pattern
3. **Session storage** tracks recent refreshes
4. **Proactive refresh** 10 minutes before expiry

## Testing

Comprehensive test coverage includes:
- Unit tests for token repository operations
- Service layer tests with mocks
- Integration tests for complete flows
- Token theft simulation tests
- Concurrent refresh handling tests

## Migration Notes

When upgrading to this implementation:
1. Run migrations `001004006_audit_auth_events_new.go` and `001004007_auth_token_family_new.go`
2. Existing tokens will continue to work until they expire
3. New tokens will use the family tracking system
4. Monitor logs for any token theft detection events