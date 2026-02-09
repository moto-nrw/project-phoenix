# Multi-Tenancy: Frontend-Implementierung

Dieses Dokument beschreibt alle Frontend-Aenderungen: Next.js Subdomain-Middleware (Rewrite Pattern), Tenant-Validation im Layout, TenantProvider, NextAuth-Konfiguration, Session-Erweiterung, SWR Cache-Keys und Env-Variablen.

**Verwandte Dokumente:**
- [01-architektur.md](01-architektur.md) - Architektur-Entscheidungen
- [03-backend.md](03-backend.md) - Backend-APIs die hier konsumiert werden
- [DEBATE.md](DEBATE.md) - Alle Diskussionspunkte und Entscheidungen

---

## 1. Next.js Middleware: Rewrite Pattern (D11)

**Entscheidung:** Subdomain-Requests werden zu `/[tenant]/...` Route-Segmenten rewritten. Vercel Platforms Starter Kit als Referenz-Implementierung. Kein Header Pattern (D11).

**Warum Rewrite statt Header:** `headers()` erzwingt Dynamic Rendering in jeder Server Component (GitHub Issues #44712, #58862). Rewrite ist type-safe (`params.tenant` statt `headers().get()`), zukunftssicher mit `next/root-params`, und das offizielle Vercel-Pattern.

```typescript
// middleware.ts — stateless, kein I/O, kein fetch()
import { NextRequest, NextResponse } from 'next/server';

const RESERVED_SUBDOMAINS = ['www', 'api', 'admin', 'operator', 'app'];
const TENANT_DOMAIN = process.env.TENANT_DOMAIN || 'localhost:3000';

export function middleware(request: NextRequest) {
    const hostname = request.headers.get('host') || '';
    const pathname = request.nextUrl.pathname;

    // Subdomain extrahieren
    let subdomain: string | null = null;

    if (hostname.includes('localhost')) {
        const match = hostname.match(/^([^.]+)\.localhost/);
        subdomain = match ? match[1] : null;
    } else {
        const parts = hostname.replace(`.${TENANT_DOMAIN}`, '');
        if (parts !== hostname) {
            subdomain = parts;
        }
    }

    // Operator-Subdomain: eigene Route-Group (bestehende Logik)
    if (subdomain === 'operator' || pathname.startsWith('/operator')) {
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

    // Tenant-Subdomain: Rewrite zu /[tenant]/... Route-Segment (D11)
    return NextResponse.rewrite(
        new URL(`/${subdomain}${pathname}`, request.url)
    );
}

export const config = {
    matcher: ['/((?!_next/static|_next/image|favicon.ico).*)'],
};
```

---

## 2. Verzeichnisstruktur (D11)

```
app/
  [tenant]/                          ← Rewrite-Target (Subdomain → Route-Segment)
    layout.tsx                       ← TenantProvider + Tenant-Validation (D5, D17)
    not-found.tsx                    ← "Diese OGS existiert nicht"
    (protected)/                     ← Bestehende Route-Group verschoben
      layout.tsx                     ← Session-Check → Login-Redirect
      dashboard/page.tsx
      students/page.tsx
      rooms/page.tsx
      groups/page.tsx
      activities/page.tsx
      settings/page.tsx
      ...                            ← 33 Pages total
    (public)/
      login/page.tsx
      invite/page.tsx
      reset-password/page.tsx
  (operator)/                        ← Bleibt an Root, kein [tenant]
    layout.tsx
    operator/login/page.tsx
    operator/announcements/page.tsx
    operator/schools/page.tsx
    ...
  api/                               ← Bleibt an Root, kein [tenant]
    auth/[...nextauth]/route.ts
    students/route.ts
    active/*/route.ts
    operator/*/route.ts
    tenant/resolve/route.ts          ← NEU: D5 Endpoint
    ...                              ← 194 Handler unveraendert
```

**API Routes bleiben wo sie sind.** Tenant kommt aus dem JWT, nicht aus der URL. Nur Pages verschieben sich nach `app/[tenant]/`.

---

## 3. Tenant-Validation im Layout (D17)

**Entscheidung:** Middleware bleibt stateless (nur Slug-Extraktion + Rewrite). Tenant-Validierung passiert im `[tenant]/layout.tsx` via `resolveTenant()` (D5-Endpoint). Invalid Tenants bekommen `notFound()` — vor der Login-Page.

```typescript
// app/[tenant]/layout.tsx
import { notFound } from 'next/navigation';
import { TenantProvider } from '@/components/tenant/tenant-provider';
import { resolveTenant } from '@/lib/tenant-api';

export default async function TenantLayout({
    params,
    children,
}: {
    params: Promise<{ tenant: string }>;
    children: React.ReactNode;
}) {
    const { tenant } = await params;
    const tenantData = await resolveTenant(tenant);

    if (!tenantData) {
        notFound(); // → app/[tenant]/not-found.tsx
    }

    return <TenantProvider value={tenantData}>{children}</TenantProvider>;
}
```

**Flow fuer unbekannten Tenant:**
```
1. User besucht nichtexistent.moto-app.de/dashboard
2. Middleware: Rewritet zu /nichtexistent/dashboard
3. [tenant]/layout.tsx: resolveTenant("nichtexistent") → null
4. notFound() → "Diese OGS existiert nicht" (OHNE Login-Page dazwischen)
```

---

## 4. TenantProvider + useTenant() Hook (D5)

### 4.1 TenantInfo Interface

```typescript
interface TenantInfo {
    tenantId: string;
    tenantSlug: string;       // aus Route-Param (params.tenant)
    tenantName: string;       // aus resolveTenant() oder Login-Response
    orgId: string;
    orgName: string;
    scope: string;            // "" | "org" | "platform"
    settings: TenantSettings; // aus platform.schools.settings JSONB
}

interface TenantSettings {
    logoUrl?: string;
    primaryColor?: string;
    [key: string]: unknown;   // Beliebig erweiterbar (JSONB)
}
```

### 4.2 useTenant() Hook

```typescript
'use client';

import { createContext, useContext } from 'react';

const TenantContext = createContext<TenantInfo | null>(null);

export function TenantProvider({
    value,
    children,
}: {
    value: TenantInfo;
    children: React.ReactNode;
}) {
    return (
        <TenantContext.Provider value={value}>
            {children}
        </TenantContext.Provider>
    );
}

export function useTenant(): TenantInfo {
    const ctx = useContext(TenantContext);
    if (!ctx) {
        throw new Error('useTenant must be used within TenantProvider');
    }
    return ctx;
}
```

### 4.3 Datenquellen

| Zeitpunkt | Was passiert |
|-----------|-------------|
| Pre-Login (public) | `GET /api/tenant/resolve?slug=...` fuer Login-Page Branding (Logo VOR dem Login) |
| Login-Response | Backend liefert `tenantName`, `orgName`, `settings` — cached im TenantContext Provider |
| Tenant-Switch | Neuer Token → neue Tenant-Info kommt automatisch mit |

---

## 5. Tenant Resolve Endpoint (D5)

```typescript
// app/api/tenant/resolve/route.ts
import { NextRequest, NextResponse } from 'next/server';
import { getServerApiUrl } from '@/lib/server-api-url';

export async function GET(request: NextRequest) {
    const slug = request.nextUrl.searchParams.get('slug');
    if (!slug) {
        return NextResponse.json({ error: 'slug required' }, { status: 400 });
    }

    const response = await fetch(
        `${getServerApiUrl()}/api/tenant/resolve?slug=${encodeURIComponent(slug)}`
    );

    if (!response.ok) {
        return NextResponse.json(null, { status: 404 });
    }

    const data = await response.json();
    return NextResponse.json(data);
}
```

**Kein Auth noetig:** Dieser Endpoint ist public — liefert nur Slug, Name, Logo, Farben. Keine sensitiven Daten.

---

## 6. NextAuth: Cookie-Domain fuer Subdomains

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
                domain: rootDomain ? `.${rootDomain}` : undefined,
                secure: isProduction,
            },
        },
    },
};
```

**Warum Wildcard-Domain?** Ohne `.moto-app.de` (mit Punkt) wuerden Cookies nur fuer die exakte Subdomain gelten. Tenant-Switch zwischen Subdomains wuerde Re-Login erfordern.

---

## 7. Session um Tenant-Info erweitern

```typescript
// NACHHER
session.user = {
    ...existing,
    tenantId: string,      // NEU: "42" (int64 -> string)
    orgId: string,         // NEU: "1"
    scope: string,         // NEU: "" | "org" | "platform"
}
```

**Type-Mapping:** Backend sendet `tenant_id: 42` (int64), Frontend speichert als `tenantId: "42"` (string). Konsistent mit dem bestehenden Muster fuer alle IDs im Projekt.

---

## 8. Login-Page: tenant_slug im Body (D6)

```typescript
// Login-Page liest Slug aus Route-Param (nicht aus Header)
export default async function LoginPage({
    params,
}: {
    params: Promise<{ tenant: string }>;
}) {
    const { tenant } = await params;

    // Login-Request sendet tenant_slug im Body (D6)
    async function handleLogin(email: string, password: string) {
        const response = await fetch('/api/auth/login', {
            method: 'POST',
            body: JSON.stringify({ email, password, tenant_slug: tenant }),
        });
        // ...
    }

    return <LoginForm onSubmit={handleLogin} />;
}
```

**Kein X-Tenant-Slug Header:** Body-Parameter ist Standard-REST (Auth0, WorkOS Pattern), leicht testbar, keine Header-Fragilitaet bei Proxies/CDNs.

---

## 9. Tenant-Switch (D4, D15)

```typescript
// lib/tenant-api.ts
export async function switchTenant(slug: string): Promise<SwitchResult> {
    const response = await apiPost('/auth/switch-tenant', {
        tenant_slug: slug,
    });
    return response.data;
}

// components/tenant/tenant-switcher.tsx
function TenantSwitcher({ availableTenants }: { availableTenants: TenantInfo[] }) {
    const router = useRouter();

    async function handleSwitch(slug: string) {
        await switchTenant(slug);
        // SWR-Cache invalidieren (Tenant-prefixed Keys aendern sich)
        mutate(() => true, undefined, { revalidate: false });
        // Redirect zur neuen Subdomain
        router.push(`https://${slug}.${TENANT_DOMAIN}/dashboard`);
    }

    return (
        // Dropdown mit verfuegbaren Tenants
    );
}
```

---

## 10. SWR Cache-Keys: Tenant-Prefix

```typescript
// NACHHER: Tenant-ID als Prefix
const { tenantId } = useTenant();
useSWR(`t${tenantId}:supervision-visits-room-1`, fetcher)
useSWR(`t${tenantId}:student-detail-42`, fetcher)
```

### Wrapper-Hook

```typescript
function useTenantSWR(key: string, fetcher: Fetcher) {
    const { tenantId } = useTenant();
    const tenantKey = tenantId ? `t${tenantId}:${key}` : key;
    return useSWR(tenantKey, fetcher);
}
```

**Warum ist das wichtig?** Ohne Tenant-Prefix koennte ein Betreuer, der zwischen OGS A und OGS B wechselt, gecachte Daten von OGS A in OGS B sehen. Das waere ein Datenleck.

---

## 11. Env-Variablen erweitern

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

## 12. URL-Struktur

```
# Tenant-spezifisch (ueber Subdomain + Rewrite):
altenberge.{TENANT_DOMAIN}/dashboard
  → Middleware rewritet zu /altenberge/dashboard
  → [tenant]/layout.tsx validiert "altenberge"
  → Next.js API Routes leiten an backend:8080 weiter (JWT enthaelt tenant_id)

# Operator (eigene Subdomain, kein Rewrite):
operator.{TENANT_DOMAIN}/operator/* → Next.js → backend:8080/operator/*

# IoT (direkt, kein Next.js):
api.{TENANT_DOMAIN}/api/iot/* → backend:8080/api/iot/*

# Root-Domain (kein Subdomain):
{TENANT_DOMAIN} → Landing Page / Tenant-Auswahl
```

---

## 13. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
| 2026-02-08 | Aktualisiert gemaess DEBATE-Entscheidungen: Rewrite Pattern statt Header (D11), TenantProvider + useTenant (D5), tenant_slug im Body (D6), [tenant]/layout.tsx Validation (D17), Tenant-Switch (D15), Tenant Resolve Endpoint (D5) |
