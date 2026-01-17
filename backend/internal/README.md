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

Hexagonal architecture migration is **complete**. All legacy paths have been migrated to the new structure.

### Core Domain (core/domain/)

All domain models, value objects, and entities are in `internal/core/domain/`:
- `auth/` - Authentication models (accounts, tokens, roles, permissions)
- `users/` - Person, staff, teacher, student, guardian models
- `education/` - Groups, teachers, substitutions
- `active/` - Real-time visit tracking and session models
- `activities/` - Activity groups, enrollments, supervisors
- And 10+ other domain packages

### Core Services (core/service/)

All business logic services in `internal/core/service/`:
- `auth/` - Authentication & authorization services
- `active/` - Visit tracking, session management, cleanup
- `users/` - Person & guardian management
- `education/` - Group management, substitutions
- And 8+ other service packages

### Adapters (adapter/)

All infrastructure implementations follow port contracts:
- `handler/http/` - Chi-based HTTP handlers (~60 resource files)
- `repository/postgres/` - BUN ORM implementations (~30 repositories)
- `mailer/` - SMTP & mock email implementations
- `middleware/` - JWT auth, device auth, permissions, wide-event logging
- `realtime/` - SSE event broadcasting hub
- `logger/` - Logrus adapter for structured logging
- `storage/` - Memory, S3, and MinIO file storage

### Ports (core/port/)

All contracts defined in `internal/core/port/`:
- Repository interfaces for all domain aggregates
- Port interfaces: EmailSender, FileStorage, Broadcaster, Logger, TokenProvider
- Permission constants & policies

## Guidelines

1. **core/domain/** - Pure Go structs, no framework dependencies, no database tags
2. **core/port/** - Interfaces only, defined by what the domain needs
3. **core/service/** - Business logic, uses ports via dependency injection
4. **adapter/** - Implementations, can import external packages (BUN, Chi, etc.)

## References

- https://threedots.tech/post/introducing-clean-architecture/
- https://dev.to/bagashiz/building-restful-api-with-hexagonal-architecture-in-go-1mij
