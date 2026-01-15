# Hexagonal Architecture (Clean Architecture)

This directory contains the application code organized according to Hexagonal Architecture
(also known as Ports and Adapters or Clean Architecture).

## Structure

```
internal/
├── core/                   ← BUSINESS LOGIC (no external dependencies!)
│   ├── domain/                 Pure entities, value objects
│   ├── port/                   Interfaces (contracts for adapters)
│   └── service/                Business logic services
│
└── adapter/                ← INFRASTRUCTURE (implements ports)
    ├── handler/                HTTP/gRPC handlers
    │   └── http/
    ├── repository/             Database implementations
    │   └── postgres/
    ├── mailer/                 Email implementation
    ├── realtime/               SSE/WebSocket implementation
    ├── middleware/             Auth, logging middleware
    └── logger/                 Logging implementation
```

## Dependency Rule

```
┌─────────────────────────────────────────────────────────┐
│                      adapter/                           │
│   handler/  repository/  mailer/  realtime/            │
│      │           │          │         │                │
│      │      implements      │    implements            │
│      ▼           ▼          ▼         ▼                │
├─────────────────────────────────────────────────────────┤
│                   core/port/                            │
│   UserRepository  EmailSender  FileStorage              │
│                      ▲                                  │
│                      │ uses                             │
├─────────────────────────────────────────────────────────┤
│                  core/service/                          │
│       AuthService    ActiveService    UserService       │
│                      ▲                                  │
│                      │ uses                             │
├─────────────────────────────────────────────────────────┤
│                   core/domain/                          │
│       User    Student    Visit    Group                 │
│         (pure entities, no dependencies)                │
└─────────────────────────────────────────────────────────┘

RULE: Arrows ALWAYS point inward!
      core/ NEVER imports adapter/
      adapter/ implements core/port/ interfaces
```

## Migration Status

This structure is being incrementally populated as we migrate from the legacy structure:

| Legacy Path                    | Target Path                              | Status   |
|-------------------------------|------------------------------------------|----------|
| `models/`                     | `internal/core/domain/`                  | Pending  |
| `models/` (interfaces)        | `internal/core/port/`                    | Pending  |
| `services/`                   | `internal/core/service/`                 | Pending  |
| `database/repositories/`      | `internal/adapter/repository/postgres/`  | Pending  |
| `api/`                        | `internal/adapter/handler/http/`         | Pending  |
| `email/`                      | `internal/adapter/mailer/`               | Pending  |
| `realtime/`                   | `internal/adapter/realtime/`             | Pending  |
| `auth/` + `middleware/`       | `internal/adapter/middleware/`           | Pending  |
| `logging/`                    | `internal/adapter/logger/`               | Pending  |

## Guidelines

1. **core/domain/** - Pure Go structs, no framework dependencies, no database tags
2. **core/port/** - Interfaces only, defined by what the domain needs
3. **core/service/** - Business logic, uses ports via dependency injection
4. **adapter/** - Implementations, can import external packages (BUN, Chi, etc.)

## References

- https://threedots.tech/post/introducing-clean-architecture/
- https://dev.to/bagashiz/building-restful-api-with-hexagonal-architecture-in-go-1mij
