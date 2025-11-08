# Authentication Flow - JWT-Based Authentication

## Overview

Project Phoenix uses **JWT (JSON Web Tokens)** for stateless authentication with
a two-token strategy:

- **Access Token**: Short-lived (15 minutes) for API requests
- **Refresh Token**: Longer-lived (1 hour) for obtaining new access tokens

## Architecture

```
┌─────────────┐
│   Frontend  │
│  (Next.js)  │
└──────┬──────┘
       │
       ↓ Login Request
┌──────────────────┐
│   NextAuth.js    │  Session stored server-side only!
│  (Session Mgmt)  │  Tokens NEVER exposed to browser
└──────┬───────────┘
       │
       ↓ POST /auth/login
┌──────────────────┐
│  Backend API     │
│  (Go Chi Router) │
└──────┬───────────┘
       │
       ↓ Validate Credentials
┌──────────────────┐
│   PostgreSQL     │
│  (auth.accounts) │
└──────────────────┘
```

## Login Flow (Detailed)

### Step 1: User Submits Credentials

```typescript
// Frontend: app/login/page.tsx
const handleLogin = async (email: string, password: string) => {
  const result = await signIn("credentials", {
    email,
    password,
    redirect: false,
  });

  if (result?.error) {
    setError("Invalid email or password");
  } else {
    router.push("/dashboard");
  }
};
```

### Step 2: NextAuth Calls Backend

```typescript
// NextAuth config: server/auth/config.ts
CredentialsProvider({
  async authorize(credentials) {
    const response = await fetch(`${API_URL}/auth/login`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        email: credentials.email,
        password: credentials.password,
      }),
    });

    if (!response.ok) return null;

    const data = await response.json();

    return {
      id: data.user.id,
      email: data.user.email,
      token: data.access_token, // 15min JWT
      refreshToken: data.refresh_token, // 1hr JWT
      roles: data.user.roles,
      isAdmin: data.user.is_admin,
    };
  },
});
```

### Step 3: Backend Validates Credentials

```go
// Backend: services/auth/auth_service.go
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResponse, error) {
    // 1. Find account by email
    account, err := s.accountRepo.FindByEmail(ctx, email)
    if err != nil {
        return nil, errors.New("invalid credentials")
    }

    // 2. Verify password (Argon2id)
    err = argon2id.ComparePasswordAndHash(password, account.PasswordHash)
    if err != nil {
        return nil, errors.New("invalid credentials")
    }

    // 3. Check if account is active
    if !account.IsActive {
        return nil, errors.New("account is disabled")
    }

    // 4. Clean up old refresh tokens (prevent accumulation)
    err = s.tokenRepo.DeleteExpiredTokens(ctx, account.ID)
    if err != nil {
        log.Printf("Warning: Failed to clean up old tokens: %v", err)
    }

    // 5. Generate JWT tokens
    accessToken, err := generateAccessToken(account, 15*time.Minute)
    if err != nil {
        return nil, err
    }

    refreshToken, err := generateRefreshToken(account, 1*time.Hour)
    if err != nil {
        return nil, err
    }

    // 6. Store refresh token in database
    err = s.tokenRepo.Create(ctx, &auth.Token{
        AccountID:   account.ID,
        Token:       refreshToken,
        Type:        "refresh",
        ExpiresAt:   time.Now().Add(1 * time.Hour),
    })
    if err != nil {
        return nil, err
    }

    // 7. Load user permissions
    permissions, err := s.permissionRepo.GetByAccountID(ctx, account.ID)
    if err != nil {
        return nil, err
    }

    return &LoginResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    900, // 15 minutes in seconds
        User: UserInfo{
            ID:       account.ID,
            Email:    account.Email,
            Roles:    account.Roles,
            IsAdmin:  account.IsAdmin,
            Permissions: permissions,
        },
    }, nil
}
```

### Step 4: NextAuth Stores Tokens in Session

```typescript
// NextAuth JWT callback: server/auth/config.ts
async jwt({ token, user }) {
  // Initial login
  if (user) {
    token.id = user.id;
    token.token = user.token;              // Access token (15min)
    token.refreshToken = user.refreshToken; // Refresh token (1hr)
    token.roles = user.roles;

    // Calculate expiry timestamps
    token.tokenExpiry = Date.now() + (15 * 60 * 1000);        // 15 min
    token.refreshTokenExpiry = Date.now() + (60 * 60 * 1000); // 1 hour
  }

  // Check if token needs refresh (1 minute before expiry)
  if (Date.now() > token.tokenExpiry - 60000) {
    return await refreshAccessToken(token);
  }

  return token;
}

// NextAuth session callback
async session({ session, token }) {
  // Expose data to session (server-side only!)
  session.user.id = token.id;
  session.user.token = token.token;  // ← NEVER sent to client!
  session.user.roles = token.roles;

  return session;
}
```

## Token Refresh Flow

### Automatic Refresh (Every 4 Minutes)

```typescript
// Frontend: app/providers.tsx
<SessionProvider
  refetchInterval={4 * 60}      // Check every 4 minutes
  refetchOnWindowFocus={true}   // Refresh on window focus
>
  {children}
</SessionProvider>
```

### Refresh Logic

```typescript
// server/auth/config.ts
async function refreshAccessToken(token: JWT): Promise<JWT> {
  try {
    // Call backend refresh endpoint
    const response = await fetch(`${API_URL}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        refresh_token: token.refreshToken,
      }),
    });

    if (!response.ok) {
      throw new Error("Refresh failed");
    }

    const refreshedTokens = await response.json();

    return {
      ...token,
      token: refreshedTokens.access_token,
      tokenExpiry: Date.now() + 15 * 60 * 1000,
    };
  } catch (error) {
    // Refresh token expired → force re-login
    return {
      ...token,
      error: "RefreshTokenError",
    };
  }
}
```

### Backend Refresh Endpoint

```go
// Backend: services/auth/auth_service.go
func (s *Service) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
    // 1. Validate refresh token signature
    claims, err := jwt.ParseWithClaims(refreshToken, &Claims{}, func(token *jwt.Token) (interface{}, error) {
        return []byte(jwtSecret), nil
    })
    if err != nil {
        return nil, errors.New("invalid refresh token")
    }

    // 2. Check if refresh token exists in database
    storedToken, err := s.tokenRepo.FindByToken(ctx, refreshToken)
    if err != nil {
        return nil, errors.New("invalid refresh token")
    }

    // 3. Check if token is expired
    if time.Now().After(storedToken.ExpiresAt) {
        return nil, errors.New("refresh token expired")
    }

    // 4. Get account
    account, err := s.accountRepo.FindByID(ctx, claims.AccountID)
    if err != nil {
        return nil, err
    }

    // 5. Generate new access token (15 min)
    accessToken, err := generateAccessToken(account, 15*time.Minute)
    if err != nil {
        return nil, err
    }

    return &TokenResponse{
        AccessToken: accessToken,
        ExpiresIn:   900, // 15 minutes
    }, nil
}
```

## API Request Authentication

### Frontend → Next.js API Route

```typescript
// Frontend component
const createGroup = async (data: CreateGroupRequest) => {
  // Call Next.js API route (no token needed - handled server-side)
  const response = await fetch("/api/groups", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(data),
  });

  return response.json();
};
```

### Next.js API Route → Backend

```typescript
// app/api/groups/route.ts
export const POST = createPostHandler(async (req, body, token) => {
  // token automatically extracted from session
  const endpoint = `/api/groups`;
  return await apiPost<BackendGroup>(endpoint, token, body);
});
```

### Route Wrapper (JWT Extraction)

```typescript
// lib/route-wrapper.ts
export function createPostHandler<T>(
  handler: (
    request: NextRequest,
    body: any,
    token: string,
    params: Record<string, unknown>
  ) => Promise<T>
) {
  return async (
    request: NextRequest,
    context: { params: Promise<Record<string, string | string[] | undefined>> }
  ) => {
    // 1. Extract session
    const session = await auth();

    if (!session?.user?.token) {
      return NextResponse.json({ error: "Unauthorized" }, { status: 401 });
    }

    // 2. Parse request body
    const body = await request.json();

    // 3. Extract params (Next.js 15: params are async)
    const contextParams = await context.params;

    try {
      // 4. Call handler with token
      const data = await handler(
        request,
        body,
        session.user.token,
        contextParams
      );

      return NextResponse.json({ success: true, data });
    } catch (error) {
      // 5. Handle 401 errors (token expired)
      if (error instanceof Error && error.message.includes("401")) {
        // Try to get refreshed session
        const updatedSession = await auth();

        // Retry with new token if available
        if (
          updatedSession?.user?.token &&
          updatedSession.user.token !== session.user.token
        ) {
          const retryData = await handler(
            request,
            body,
            updatedSession.user.token,
            contextParams
          );
          return NextResponse.json({ success: true, data: retryData });
        }

        // Token refresh failed → return 401
        return NextResponse.json(
          { error: "Token expired", code: "TOKEN_EXPIRED" },
          { status: 401 }
        );
      }

      throw error;
    }
  };
}
```

### Backend JWT Validation

```go
// Backend: auth/jwt/middleware.go
func Verifier(w http.ResponseWriter, r *http.Request, next http.Handler) {
    // 1. Extract token from Authorization header
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        http.Error(w, "Missing authorization header", http.StatusUnauthorized)
        return
    }

    parts := strings.Split(authHeader, " ")
    if len(parts) != 2 || parts[0] != "Bearer" {
        http.Error(w, "Invalid authorization header", http.StatusUnauthorized)
        return
    }

    tokenString := parts[1]

    // 2. Parse and validate JWT
    claims := &Claims{}
    token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
        // Verify signing method
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method")
        }
        return []byte(jwtSecret), nil
    })

    if err != nil || !token.Valid {
        http.Error(w, "Invalid token", http.StatusUnauthorized)
        return
    }

    // 3. Check expiration
    if time.Now().After(time.Unix(claims.ExpiresAt, 0)) {
        http.Error(w, "Token expired", http.StatusUnauthorized)
        return
    }

    // 4. Inject claims into context
    ctx := context.WithValue(r.Context(), "account_id", claims.AccountID)
    ctx = context.WithValue(ctx, "permissions", claims.Permissions)

    next.ServeHTTP(w, r.WithContext(ctx))
}
```

## Security Considerations

### 1. Tokens Never Exposed to Client

❌ **Wrong** (Insecure):

```typescript
// DON'T DO THIS!
localStorage.setItem("token", session.user.token);
```

✅ **Correct** (Secure):

```typescript
// Tokens stored in NextAuth session (server-side only)
const session = await auth(); // Server-side
const token = session?.user?.token; // Never sent to browser
```

### 2. Token Expiry Configuration

```go
// Backend: config/auth.go
AccessTokenExpiry  = 15 * time.Minute  // Short-lived for security
RefreshTokenExpiry = 1 * time.Hour     // Longer for UX
```

**Rationale**:

- **15 minutes**: Limits exposure window if token is compromised
- **1 hour**: Balances security with user experience (fewer re-logins)

### 3. Password Hashing (Argon2id)

```go
// Backend: auth/password/argon2id.go
type Params struct {
    Memory      uint32 // 64 MB
    Iterations  uint32 // 3 iterations
    Parallelism uint8  // 2 parallel threads
    SaltLength  uint32 // 16 bytes
    KeyLength   uint32 // 32 bytes
}

func HashPassword(password string) (string, error) {
    salt := make([]byte, params.SaltLength)
    _, err := rand.Read(salt)
    if err != nil {
        return "", err
    }

    hash := argon2.IDKey(
        []byte(password),
        salt,
        params.Iterations,
        params.Memory,
        params.Parallelism,
        params.KeyLength,
    )

    // Encode: $argon2id$v=19$m=65536,t=3,p=2$[salt]$[hash]
    encoded := base64.RawStdEncoding.EncodeToString(salt) + "$" +
               base64.RawStdEncoding.EncodeToString(hash)

    return encoded, nil
}
```

**Why Argon2id?**

- Winner of Password Hashing Competition (2015)
- Resistant to GPU/ASIC attacks
- Memory-hard (prevents parallel attacks)

### 4. Token Cleanup

```go
// Prevent token accumulation in database
func (s *Service) Login(ctx context.Context, email, password string) {
    // Clean up expired tokens BEFORE creating new ones
    err := s.tokenRepo.DeleteExpiredTokens(ctx, account.ID)
    if err != nil {
        log.Printf("Warning: Failed to clean up old tokens: %v", err)
    }

    // Create new refresh token
    // ...
}
```

## Logout Flow

```typescript
// Frontend
const handleLogout = async () => {
  await signOut({ redirect: true, callbackUrl: "/login" });
};
```

```go
// Backend: services/auth/auth_service.go
func (s *Service) Logout(ctx context.Context, accountID int64, refreshToken string) error {
    // Delete refresh token from database
    err := s.tokenRepo.DeleteByToken(ctx, refreshToken)
    if err != nil {
        return err
    }

    // Optionally: Blacklist access token (requires Redis or similar)
    // Not implemented: Access tokens expire in 15 minutes anyway

    return nil
}
```

## Troubleshooting

### Issue: "Token expired" errors

**Cause**: Access token expired before refresh could occur

**Solution**: Ensure `SessionProvider` refetch interval < access token expiry:

```typescript
<SessionProvider refetchInterval={4 * 60}>  // 4 min < 15 min
```

### Issue: User logged out unexpectedly

**Cause**: Refresh token expired

**Solution**:

1. Check refresh token expiry setting (default: 1 hour)
2. Increase if users complain about frequent re-logins
3. Balance security vs UX

### Issue: "Invalid token" on API requests

**Cause**: JWT secret mismatch between frontend and backend

**Solution**:

1. Verify `AUTH_JWT_SECRET` in backend `.env`
2. Verify `NEXTAUTH_SECRET` in frontend `.env.local`
3. Ensure both environments use the same secret

---

**See Also**:

- [Authorization Model](authorization-model.md) - Permission system
- [GDPR Compliance](gdpr-compliance.md) - Data protection
- [ADR-004: JWT Tokens](../adr/004-jwt-tokens.md) - Decision rationale
