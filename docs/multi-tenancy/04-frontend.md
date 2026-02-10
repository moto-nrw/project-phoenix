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

## 13. Cookie-Security: Wildcard Domain Hardening (09-H4)

**Problem:** Cookie mit `domain: .moto-app.de` bedeutet: XSS auf einer Subdomain kann Sessions fuer ALLE Subdomains stehlen. Das ist inherent zum Subdomain-Cookie-Design — ohne Wildcard-Cookie muessten User bei jedem Tenant-Switch neu einloggen.

**Mitigations (Defense-in-Depth):**

### 13.1 Content Security Policy (CSP) per Subdomain

```typescript
// middleware.ts — CSP-Header fuer jede Response
const cspHeader = [
    "default-src 'self'",
    "script-src 'self' 'nonce-${nonce}'",      // Kein inline-script
    "style-src 'self' 'unsafe-inline'",         // Tailwind braucht inline styles
    "img-src 'self' data: blob:",
    "connect-src 'self' wss:",                  // SSE/WebSocket
    "frame-ancestors 'none'",                   // Kein Embedding
    "base-uri 'self'",                          // Kein Base-Tag Hijacking
    "form-action 'self'",                       // Kein Form-Redirect
].join('; ');
```

### 13.2 Cookie-Schutzschichten

| Schicht | Was sie schuetzt | Status |
|---------|-----------------|--------|
| `HttpOnly` | Kein JavaScript-Zugriff auf Cookie | Bereits gesetzt |
| `SameSite=Lax` | Kein CSRF via Cross-Site POST | Bereits gesetzt |
| `Secure` | Nur ueber HTTPS | Bereits gesetzt (Production) |
| `__Secure-` Prefix | Browser erzwingt Secure-Flag | Bereits gesetzt |
| JWT `tenant_id` | Cookie-Diebstahl gibt nur Zugang zum aktuellen Tenant | Architektur-inhaerent |
| CSP | Verhindert XSS (primaerer Angriffsvektor) | NEU — muss implementiert werden |

### 13.3 Warum `__Host-` Prefix NICHT funktioniert

`__Host-` Prefix wuerde Cookie an exakte Origin binden (kein `domain` Attribut). Aber dann funktioniert Tenant-Switch nicht — Cookie von `altenberge.moto-app.de` waere bei `greven.moto-app.de` unsichtbar.

**Akzeptiertes Restrisiko:** Wildcard-Cookie ist noetig fuer UX. XSS-Praevention via CSP ist die primaere Verteidigung. Subdomain-Takeover-Monitoring als operationale Massnahme.

---

## 14. Login-Validierung: Slug vs Subdomain (09-H5)

**Problem:** Login sendet `tenant_slug` aus URL-Params im Body. Ohne Validierung:
1. User besucht `altenberge.moto-app.de/login`
2. Attacker intercepted Request, aendert Body zu `{ tenant_slug: "greven" }`
3. Backend gibt JWT fuer Greven zurueck
4. UI zeigt Altenberge-Branding, Daten kommen von Greven

**Fix: Backend validiert Slug gegen Origin-Header**

```go
// backend/api/auth/login_handler.go
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
    var req LoginRequest
    // ... parse body ...

    // NEU: Slug-Origin-Validierung (09-H5)
    origin := r.Header.Get("Origin")
    if origin != "" {
        originURL, err := url.Parse(origin)
        if err == nil {
            expectedSlug := extractSubdomain(originURL.Hostname())
            if expectedSlug != "" && expectedSlug != req.TenantSlug {
                http.Error(w, "tenant_slug does not match origin", http.StatusBadRequest)
                return
            }
        }
    }

    // ... normaler Login-Flow ...
}
```

**Warum Origin statt Referer:** `Origin` ist bei POST-Requests immer gesetzt (Fetch Spec). `Referer` kann vom Browser oder Proxy gekuerzt werden. Fallback: Wenn `Origin` fehlt (alte Browser, Proxy), Login trotzdem erlauben — die CSP verhindert XSS-basierte Angriffe.

---

## 15. NextAuth Session: Tenant-Felder (09-H6)

**Problem:** Aktuelles `JwtPayload` Interface hat keine Tenant-Felder. Nach Token-Refresh (alle 15 Minuten) geht der Tenant-Context verloren.

### 15.1 Erweitertes JwtPayload

```typescript
// server/auth/config.ts
interface JwtPayload {
    // Bestehende Felder:
    id: string | number;
    sub?: string;
    username?: string;
    first_name?: string;
    last_name?: string;
    email?: string;
    roles?: string[];
    is_admin?: boolean;

    // NEU: Tenant-Felder (D5, D12, D13 rev)
    tenant_id?: number;
    tenant_slug?: string;
    org_id?: number;
    scope?: string;            // "" | "org" | "platform"
    permissions?: string[];    // Per-Tenant Permissions (D13 rev)
}
```

### 15.2 parseJwtPayload Erweiterung

```typescript
function parseJwtPayload(token: string): JwtPayload {
    const payload = jwtDecode<JwtPayload>(token);
    return {
        ...payload,
        // Sicherstellen dass Tenant-Felder vorhanden sind
        tenant_id: payload.tenant_id ?? undefined,
        tenant_slug: payload.tenant_slug ?? undefined,
        org_id: payload.org_id ?? undefined,
        scope: payload.scope ?? '',
        permissions: payload.permissions ?? [],
    };
}
```

### 15.3 Token-Refresh bewahrt Tenant-Context (D12)

```typescript
// auth-api.ts — Refresh-Flow
async function refreshToken(refreshToken: string): Promise<TokenPair> {
    // Backend re-validiert account_tenants Membership (D12)
    // und gibt neuen JWT mit aktuellen tenant_id + permissions zurueck
    const response = await fetch('/api/auth/refresh', {
        method: 'POST',
        body: JSON.stringify({ refresh_token: refreshToken }),
    });
    // Neuer JWT enthaelt garantiert tenant_id + tenant_slug + permissions
    return response.json();
}
```

---

## 16. SWR + Session Cache: Vollstaendige Isolation (09-H7)

### 16.1 useTenantSWR (bereits in §10)

Alle 821 SWR-Calls muessen `useTenantSWR` nutzen. Migration:

```typescript
// VORHER (821 Stellen)
useSWR('/api/students', fetcher)

// NACHHER
useTenantSWR('/api/students', fetcher)
// → interner Key: "t42:/api/students"
```

**CI-Durchsetzung:** ESLint-Regel blockiert direkten `useSWR`-Import (analog zur `router.push`-Regel in §17):

```javascript
// eslint: no-restricted-imports
"no-restricted-imports": ["error", {
    "paths": [{
        "name": "swr",
        "importNames": ["default"],
        "message": "Use useTenantSWR from @/lib/tenant-swr instead of useSWR directly. Direct useSWR bypasses tenant cache isolation."
    }]
}]
```

### 16.2 Session Cache Invalidierung bei Tenant-Switch

**Problem:** `session-cache.ts` cached die Session 10 Sekunden lang auf Modul-Ebene. Nach Tenant-Switch liefert der Cache den ALTEN JWT mit dem ALTEN `tenant_id`.

```typescript
// lib/session-cache.ts — NEU: Tenant-Awareness
let cachedSession: CachedSession | null = null;
let cachedTenantId: string | null = null;

export function getCachedSession(currentTenantId: string): Session | null {
    if (cachedTenantId !== currentTenantId) {
        // Tenant gewechselt → Cache invalidieren
        cachedSession = null;
        cachedTenantId = currentTenantId;
        return null;
    }
    if (cachedSession && Date.now() - cachedSession.timestamp < 10_000) {
        return cachedSession.session;
    }
    return null;
}
```

### 16.3 SWR-Cache Invalidierung bei Tenant-Switch

Bereits in §9 dokumentiert (`mutate(() => true, undefined, { revalidate: false })`). Zusaetzlich:

```typescript
async function handleSwitch(slug: string) {
    await switchTenant(slug);

    // 1. SWR-Cache komplett invalidieren
    mutate(() => true, undefined, { revalidate: false });

    // 2. Session-Cache invalidieren
    clearSessionCache();

    // 3. Redirect zur neuen Subdomain (neuer Origin = neuer localStorage)
    window.location.href = `https://${slug}.${TENANT_DOMAIN}/dashboard`;
}
```

**Warum `window.location.href` statt `router.push`:** Hard-Navigation zu einer anderen Subdomain ist ein Origin-Wechsel. Der Browser erstellt einen neuen JS-Context — kein stale Cache, kein stale State.

### 16.4 Browser HTTP Cache

```typescript
// lib/api.ts — Cache-Control Headers
const axiosInstance = axios.create({
    headers: {
        'Cache-Control': 'no-store',  // Kein Browser-Cache fuer API-Responses
    },
});
```

API-Responses mit Tenant-Daten duerfen NICHT im Browser-Cache landen. `no-store` ist restriktiver als `no-cache` (kein Speichern, nicht mal mit Revalidierung).

---

## 17. Redirect-Pfade: Tenant-Prefix Pattern (09-H8)

**Problem:** 40+ Stellen nutzen hardcoded Redirects (`"/dashboard"`, `"/rooms"`, `"/"`) ohne Tenant-Prefix. Nach der Migration zu `[tenant]/...` brechen alle.

### 17.1 Tenant-Router Helper

```typescript
// lib/tenant-router.ts
import { useTenant } from '@/components/tenant/tenant-provider';
import { useRouter } from 'next/navigation';

export function useTenantRouter() {
    const { tenantSlug } = useTenant();
    const router = useRouter();

    return {
        push: (path: string) => router.push(`/${tenantSlug}${path}`),
        replace: (path: string) => router.replace(`/${tenantSlug}${path}`),
    };
}

// Server-side Redirect Helper
export function tenantRedirect(tenantSlug: string, path: string): string {
    return `/${tenantSlug}${path}`;
}
```

### 17.2 Betroffene Stellen (Audit)

| Datei | Aktuell | Nachher |
|-------|---------|---------|
| `lib/api.ts:471` | `window.location.href = "/"` | `window.location.href = "/${slug}/login"` |
| `lib/auth-api.ts:169` | `window.location.href = "/"` | `window.location.href = "/${slug}/login"` |
| `server/auth/config.ts:403` | `pages: { signIn: "/" }` | `pages: { signIn: "/${slug}/login" }` |
| `dashboard/page.tsx` | `router.replace("/")` | `tenantRouter.replace("/dashboard")` |
| `sidebar.tsx` | `router.push("/ogs-groups")` | `tenantRouter.push("/ogs-groups")` |
| `rooms/page.tsx` | `router.push("/rooms/${id}")` | `tenantRouter.push("/rooms/${id}")` |

**Vollstaendige Migration:** Alle `router.push("/"...)` und `window.location.href = "/"` Aufrufe muessen auf `useTenantRouter()` bzw. `tenantRedirect()` umgestellt werden. CI-Lint-Rule um neue hardcoded Redirects zu verhindern:

```javascript
// eslint: no-restricted-syntax rule
"no-restricted-syntax": [
    "error",
    {
        selector: "CallExpression[callee.property.name='push'][arguments.0.value=/^\\/[^[]/]",
        message: "Use useTenantRouter().push() instead of router.push() with hardcoded paths"
    }
]
```

---

## 18. Tenant-Switch: NextAuth Session Update (09-M3)

**Problem:** `switchTenant()` gibt einen neuen JWT zurueck, aber speichert ihn nicht in der NextAuth Session. Der Wildcard-Cookie traegt die ALTE Session zur neuen Subdomain. UI zeigt neuen Tenant, API-Calls nutzen alten JWT.

**Fix:** Tenant-Switch muss NextAuth Session updaten BEVOR der Redirect passiert:

```typescript
async function handleSwitch(slug: string) {
    // 1. Backend gibt neuen JWT zurueck
    const { access_token, refresh_token } = await switchTenant(slug);

    // 2. NextAuth Session mit neuem Token updaten
    // signIn("credentials") mit dem neuen Token
    await signIn('credentials', {
        redirect: false,
        access_token,
        refresh_token,
    });

    // 3. Caches invalidieren (§16.3)
    mutate(() => true, undefined, { revalidate: false });
    clearSessionCache();

    // 4. Hard-Navigate zur neuen Subdomain
    window.location.href = `https://${slug}.${TENANT_DOMAIN}/dashboard`;
}
```

**Alternativ (einfacher):** Da `window.location.href` einen Origin-Wechsel macht, reicht es wenn das Backend den neuen JWT direkt als Cookie setzt (via `Set-Cookie` Header in der Switch-Response). Die neue Subdomain empfaengt dann automatisch den neuen Cookie.

---

## 19. Lokale Entwicklungsumgebung fuer Subdomains (06-#8)

### 19.1 Browser-Kompatibilitaet

Moderne Browser unterstuetzen `*.localhost` nativ — kein `/etc/hosts`-Eintrag noetig:

| Browser | `*.localhost` Support | Mindestversion |
|---------|----------------------|----------------|
| Chrome | Ja | 63+ (Dez 2017) |
| Firefox | Ja | 84+ (Dez 2020) |
| Edge | Ja | 79+ (Chromium-basiert) |
| Safari | **Nein** | — (erfordert `/etc/hosts`) |

### 19.2 URLs fuer lokale Entwicklung

```bash
# Tenant-spezifische Pages:
http://school-a.localhost:3000/login
http://school-b.localhost:3000/dashboard

# Root-Domain (Tenant-Auswahl / Landing):
http://localhost:3000/

# Operator:
http://operator.localhost:3000/operator/login
# Alternativ ohne Subdomain (bestehende Route):
http://localhost:3000/operator/login
```

### 19.3 Environment-Setup (dev.env / .env.local)

```bash
# backend/dev.env — CORS fuer Wildcard-Localhost
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://*.localhost:3000

# frontend/.env.local
TENANT_DOMAIN=localhost:3000
NEXT_PUBLIC_API_URL=http://localhost:8080
# API_URL: nicht setzen (localhost:8080 fuer Server + Client)
```

### 19.4 Seed-Daten fuer lokale Tenants

```bash
# Nach Migration: Default-Tenant + Test-Tenant erstellen
go run main.go seed

# Seeds erstellen automatisch:
# - Organization "Traeger Test" (ID=1)
# - School "school-a" (ID=1, Subdomain: school-a)
# - School "school-b" (ID=2, Subdomain: school-b)
# Damit sind http://school-a.localhost:3000/ und http://school-b.localhost:3000/ sofort nutzbar
```

### 19.5 Safari-Workaround (optional)

Safari erkennt `*.localhost` nicht als Loopback. Workaround:

```bash
# /etc/hosts (nur fuer Safari-Entwickler)
127.0.0.1  school-a.localhost school-b.localhost operator.localhost
```

### 19.6 Docker Compose

Keine Aenderungen an `docker-compose.yml` noetig. Das Frontend laeuft auf Port 3000 und empfaengt Subdomains via Browser → `Host` Header → Middleware. Fuer containerisierte Entwicklung (Frontend im Container):

```yaml
# docker-compose.override.yml (optional)
frontend:
  environment:
    TENANT_DOMAIN: "localhost:3000"
```

---

## 20. Aenderungshistorie

| Datum | Aenderung |
|-------|-----------|
| 2026-02-08 | Initiale Version basierend auf vollstaendiger Codebase-Analyse |
| 2026-02-08 | Aktualisiert gemaess DEBATE-Entscheidungen: Rewrite Pattern statt Header (D11), TenantProvider + useTenant (D5), tenant_slug im Body (D6), [tenant]/layout.tsx Validation (D17), Tenant-Switch (D15), Tenant Resolve Endpoint (D5) |
| 2026-02-10 | Security-Sektionen ergaenzt: Cookie Hardening mit CSP (§13, 09-H4), Login Slug-Origin-Validierung (§14, 09-H5), NextAuth JwtPayload Tenant-Felder (§15, 09-H6), SWR + Session Cache Isolation (§16, 09-H7), Tenant-Router Helper fuer Redirects (§17, 09-H8), NextAuth Session Update bei Tenant-Switch (§18, 09-M3) |
| 2026-02-10 | Lokale Entwicklungsumgebung dokumentiert (§19, 06-#8): Browser-Kompatibilitaet, URLs, Environment-Setup, Seed-Daten, Safari-Workaround, Docker Compose. |
