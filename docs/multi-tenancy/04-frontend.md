# Multi-Tenancy: Frontend-Implementierung

Dieses Dokument beschreibt alle Frontend-Aenderungen: Next.js Subdomain-Middleware, NextAuth-Konfiguration, Session-Erweiterung, Route-Wrapper, SWR Cache-Keys und Env-Variablen.

**Verwandte Dokumente:**
- [01-architektur.md](01-architektur.md) - Architektur-Entscheidungen
- [03-backend.md](03-backend.md) - Backend-APIs die hier konsumiert werden

---

## 1. Next.js Middleware: Subdomain-Extraktion

```typescript
// middleware.ts - KOMPLETT NEU
import { NextRequest, NextResponse } from 'next/server';

const TENANT_DOMAIN = process.env.TENANT_DOMAIN || 'localhost:3000';
const RESERVED_SUBDOMAINS = ['operator', 'api', 'www'];

export function middleware(request: NextRequest) {
    const hostname = request.headers.get('host') || '';
    const pathname = request.nextUrl.pathname;

    // Subdomain extrahieren
    let subdomain: string | null = null;

    if (hostname.includes('localhost')) {
        // Dev: subdomain.localhost:3000
        const match = hostname.match(/^([^.]+)\.localhost/);
        subdomain = match ? match[1] : null;
    } else {
        // Prod: subdomain.{TENANT_DOMAIN}
        const parts = hostname.replace(`.${TENANT_DOMAIN}`, '');
        if (parts !== hostname) {
            subdomain = parts;
        }
    }

    // Operator-Subdomain: eigene Route-Group
    if (subdomain === 'operator') {
        if (!pathname.startsWith('/operator/login')) {
            const token = request.cookies.get('phoenix-operator-token');
            if (!token?.value) {
                return NextResponse.redirect(new URL('/operator/login', request.url));
            }
        }
        return NextResponse.next();
    }

    // Root-Domain (kein Subdomain): Landing/Tenant-Auswahl
    if (!subdomain || subdomain === 'www') {
        return NextResponse.next();
    }

    // Reservierte Subdomains blocken
    if (RESERVED_SUBDOMAINS.includes(subdomain)) {
        return NextResponse.next();
    }

    // Tenant-Subdomain: Slug als Header weiterreichen
    const response = NextResponse.next();
    response.headers.set('x-tenant-slug', subdomain);
    return response;
}

export const config = {
    matcher: ['/((?!_next/static|_next/image|favicon.ico).*)'],
};
```

---

## 2. NextAuth: Cookie-Domain fuer Subdomains

```typescript
// server/auth/config.ts - Cookie-Konfiguration erweitern
const isProduction = process.env.NODE_ENV === 'production';
const rootDomain = process.env.TENANT_DOMAIN; // z.B. "moto-app.de"

export const authConfig = {
    // ...existing config...
    cookies: {
        sessionToken: {
            name: isProduction
                ? '__Secure-next-auth.session-token'
                : 'next-auth.session-token',
            options: {
                httpOnly: true,
                sameSite: 'lax' as const,
                path: '/',
                // Wildcard-Domain fuer alle Subdomains
                domain: rootDomain ? `.${rootDomain}` : undefined,
                secure: isProduction,
            },
        },
    },
};
```

**Warum Wildcard-Domain?** Ohne `.moto-app.de` (mit Punkt) wuerden Cookies nur fuer die exakte Subdomain gelten. Wenn ein User von `altenberge.moto-app.de` auf `greven.moto-app.de` wechselt (z.B. Betreuer an 2 OGS), muesste er sich neu einloggen.

---

## 3. Session um Tenant-Info erweitern

```typescript
// VORHER
session.user = {
    id: string,
    name: string,
    email: string,
    token: string,
    refreshToken: string,
    roles: string[],
    firstName: string,
    isAdmin: boolean,
}

// NACHHER
session.user = {
    ...existing,
    tenantId: string,      // NEU: "1" (int64 -> string)
    orgId: string,         // NEU: "1"
    scope: string,         // NEU: "" | "org" | "platform"
}
```

**Type-Mapping:** Backend sendet `tenant_id: 1` (int64), Frontend speichert als `tenantId: "1"` (string). Konsistent mit dem bestehenden Muster fuer alle IDs im Projekt.

---

## 4. Route-Wrapper: Tenant-Header automatisch

```typescript
// lib/route-wrapper.ts - X-Tenant-Slug automatisch setzen
async function forwardToBackend(request: Request, token: string, path: string) {
    const headers = new Headers({
        'Authorization': `Bearer ${token}`,
        'Content-Type': 'application/json',
    });

    // NEU: Tenant-Slug aus Request-Header (gesetzt von Next.js Middleware)
    const tenantSlug = request.headers.get('x-tenant-slug');
    if (tenantSlug) {
        headers.set('X-Tenant-Slug', tenantSlug);
    }

    return fetch(`${getServerApiUrl()}${path}`, { headers });
}
```

**Wann wird `X-Tenant-Slug` benoetigt?**
- Beim Login-Request (noch kein JWT vorhanden)
- Fuer alle weiteren Requests kommt die `tenant_id` aus dem JWT

---

## 5. SWR Cache-Keys: Tenant-Prefix

```typescript
// VORHER
useSWR('supervision-visits-room-1', fetcher)
useSWR('student-detail-42', fetcher)

// NACHHER: Tenant-ID als Prefix
const { tenantId } = useSession();
useSWR(`t${tenantId}:supervision-visits-room-1`, fetcher)
useSWR(`t${tenantId}:student-detail-42`, fetcher)
```

### 5.1 Wrapper-Hook (empfohlen)

```typescript
function useTenantSWR(key: string, fetcher: Fetcher) {
    const { data: session } = useSession();
    const tenantKey = session?.user?.tenantId
        ? `t${session.user.tenantId}:${key}`
        : key;
    return useSWR(tenantKey, fetcher);
}
```

**Warum ist das wichtig?** Ohne Tenant-Prefix koennte ein Betreuer, der zwischen OGS A und OGS B wechselt, gecachte Daten von OGS A in OGS B sehen. Das waere ein Datenleck.

---

## 6. Env-Variablen erweitern

```typescript
// env.js - Neue Variablen
const env = createEnv({
    server: {
        // ...existing...
        TENANT_DOMAIN: z.string().optional(),  // NEU: z.B. "moto-app.de"
    },
    client: {
        // ...existing...
    },
});
```

---

## 7. URL-Struktur

```
# Tenant-spezifisch (ueber Subdomain geroutet):
altenberge.{TENANT_DOMAIN}/api/students -> Next.js -> backend:8080/api/students

# Operator (eigene Subdomain):
operator.{TENANT_DOMAIN}/api/operator/* -> Next.js -> backend:8080/operator/*

# IoT (direkt, kein Next.js):
api.{TENANT_DOMAIN}/api/iot/* -> backend:8080/api/iot/*

# Root-Domain (kein Subdomain):
{TENANT_DOMAIN} -> Landing Page / Tenant-Auswahl
```

---

## 8. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
