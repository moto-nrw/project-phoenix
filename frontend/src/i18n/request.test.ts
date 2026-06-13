import { describe, it, expect, vi, beforeEach } from "vitest";
import { LOCALE_COOKIE_NAME, LOCALE_SCOPE_HEADER } from "./locales";

// getRequestConfig normally wraps the callback for the next-intl runtime. For
// the test we want the raw callback, so the mock returns it untouched — the
// default export of request.ts then IS the resolver we can invoke directly.
vi.mock("next-intl/server", () => ({
  getRequestConfig: (cb: unknown) => cb,
}));

// next/headers needs a request context that doesn't exist under the test
// runner; back it with plain maps we control per test.
const cookieStore = new Map<string, string>();
const headerStore = new Map<string, string>();
vi.mock("next/headers", () => ({
  cookies: () =>
    Promise.resolve({
      get: (name: string) =>
        cookieStore.has(name) ? { value: cookieStore.get(name) } : undefined,
    }),
  headers: () =>
    Promise.resolve({
      get: (name: string) => headerStore.get(name) ?? null,
    }),
}));

// The default export is the async resolver returned by getRequestConfig.
type Resolver = () => Promise<{
  locale: string;
  messages: Record<string, unknown>;
}>;

async function loadResolver(): Promise<Resolver> {
  const mod = await import("./request");
  return mod.default as unknown as Resolver;
}

beforeEach(() => {
  cookieStore.clear();
  headerStore.clear();
});

describe("request.ts getRequestConfig resolver", () => {
  it("stays on the default locale and de catalog when not a localized surface", async () => {
    // No localize header → staff/operator surface.
    cookieStore.set(LOCALE_COOKIE_NAME, "en");
    headerStore.set("accept-language", "en-US");

    const resolve = await loadResolver();
    const { locale, messages } = await resolve();

    expect(locale).toBe("de");
    // de catalog has a `language` namespace; assert it loaded.
    expect(messages.language).toBeDefined();
  });

  it("resolves the cookie locale and merges its catalog on a localized surface", async () => {
    headerStore.set(LOCALE_SCOPE_HEADER, "1");
    cookieStore.set(LOCALE_COOKIE_NAME, "en");

    const resolve = await loadResolver();
    const { locale, messages } = await resolve();

    expect(locale).toBe("en");
    // Merged catalog: a top-level namespace from the de fallback must still be
    // present even if the en catalog structures it identically.
    expect(messages.language).toBeDefined();
  });

  it("falls back to Accept-Language when localized with no cookie", async () => {
    headerStore.set(LOCALE_SCOPE_HEADER, "1");
    headerStore.set("accept-language", "ru,en;q=0.8");

    const resolve = await loadResolver();
    const { locale } = await resolve();

    expect(locale).toBe("ru");
  });
});
