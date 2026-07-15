import { describe, expect, it } from "vitest";
import { NextRequest } from "next/server";
import {
  canonicalForwardedFor,
  getClientForwardHeaders,
  hostnameFromAuthority,
} from "./client-headers.server";

describe("getClientForwardHeaders", () => {
  it("uses the request host for the frontend origin on localhost subdomains", () => {
    const request = new NextRequest(
      "http://localhost:3000/api/auth/passkeys/login/options",
      {
        headers: {
          "x-moto-original-host": "school-a.localhost:3000",
          "user-agent": "browser",
          "x-forwarded-for": "203.0.113.10",
        },
      },
    );

    expect(getClientForwardHeaders(request)).toMatchObject({
      "X-Moto-Frontend-Origin": "http://school-a.localhost:3000",
      "X-Forwarded-For": "203.0.113.10",
      "User-Agent": "browser",
    });
    expect(getClientForwardHeaders(request)).not.toHaveProperty("X-Real-IP");
  });

  it("forwards the rightmost client IP from a forwarded-for chain", () => {
    const request = new NextRequest(
      "http://localhost:3000/api/auth/passkeys/login/options",
      {
        headers: {
          "x-forwarded-for": "203.0.113.10, 172.20.0.4",
        },
      },
    );

    expect(getClientForwardHeaders(request)).toMatchObject({
      "X-Forwarded-For": "172.20.0.4",
    });
    expect(getClientForwardHeaders(request)).not.toHaveProperty("X-Real-IP");
  });

  it("falls back to x-real-ip when forwarded-for is absent", () => {
    const request = new NextRequest(
      "http://localhost:3000/api/auth/passkeys/login/options",
      {
        headers: {
          "x-real-ip": "198.51.100.25",
        },
      },
    );

    expect(getClientForwardHeaders(request)).toMatchObject({
      "X-Forwarded-For": "198.51.100.25",
    });
  });

  it("honors x-forwarded-proto when building the frontend origin", () => {
    const request = new NextRequest(
      "http://localhost:3000/api/auth/passkeys/login/options",
      {
        headers: {
          "x-moto-original-host": "school-a.moto-app.de",
          "x-moto-original-proto": "https",
        },
      },
    );

    expect(getClientForwardHeaders(request)["X-Moto-Frontend-Origin"]).toBe(
      "https://school-a.moto-app.de",
    );
  });
});

describe("hostnameFromAuthority", () => {
  it.each([
    ["Parents.Example.Test", "parents.example.test"],
    ["parents.example.test:443", "parents.example.test"],
    ["parents.example.test.", "parents.example.test"],
    ["127.0.0.1:3000", "127.0.0.1"],
    ["[2001:0db8::1]", "2001:db8::1"],
    ["[2001:db8::1]:443", "2001:db8::1"],
  ])("normalizes %s to %s", (authority, expected) => {
    expect(hostnameFromAuthority(authority)).toBe(expected);
  });

  it.each([
    "",
    " parents.example.test",
    "parents.example.test ",
    "parent user.example.test",
    "parent@parents.example.test",
    "parents.example.test/path",
    "parents.example.test\\path",
    "parents.example.test?query",
    "parents.example.test#fragment",
    "parents.example.test:",
    "parents.example.test:invalid",
    "parents.example.test:65536",
    "https://parents.example.test",
    "parents..example.test",
    "parents_example.test",
    "2001:db8::1",
    "[2001:db8::1",
  ])("rejects malformed authority %s", (authority) => {
    expect(hostnameFromAuthority(authority)).toBeNull();
  });
});

describe("canonicalForwardedFor", () => {
  it("uses the rightmost non-empty forwarded-for entry", () => {
    expect(
      canonicalForwardedFor(
        new Headers({
          "x-forwarded-for": " , 203.0.113.10, 172.20.0.4",
          "x-real-ip": "198.51.100.25",
        }),
      ),
    ).toBe("172.20.0.4");
  });

  it("rejects spoofed leftmost entries by selecting the rightmost valid entry", () => {
    expect(
      canonicalForwardedFor(
        new Headers({
          "x-forwarded-for": "spoofed, 203.0.113.10",
        }),
      ),
    ).toBe("203.0.113.10");
  });

  it("fails closed when the rightmost forwarded-for entry is invalid", () => {
    expect(
      canonicalForwardedFor(
        new Headers({
          "x-forwarded-for": "203.0.113.10, not-an-ip",
          "x-real-ip": "198.51.100.25",
        }),
      ),
    ).toBe("");
  });

  it("omits invalid x-real-ip fallback values", () => {
    expect(
      canonicalForwardedFor(
        new Headers({
          "x-real-ip": "203.0.113.10:1234",
        }),
      ),
    ).toBe("");
  });

  it("returns an empty string when no client IP headers are present", () => {
    expect(canonicalForwardedFor(new Headers())).toBe("");
    expect(canonicalForwardedFor(null)).toBe("");
  });
});
