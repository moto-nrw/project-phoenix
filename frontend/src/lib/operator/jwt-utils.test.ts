import { describe, it, expect } from "vitest";
import { extractJwtExpiry } from "./jwt-utils";

function makeJwt(payload: Record<string, unknown>): string {
  const header = Buffer.from(
    JSON.stringify({ alg: "HS256", typ: "JWT" }),
  ).toString("base64url");
  const body = Buffer.from(JSON.stringify(payload)).toString("base64url");
  const signature = "fake-signature";
  return `${header}.${body}.${signature}`;
}

describe("extractJwtExpiry", () => {
  it("returns expiry in milliseconds from valid JWT", () => {
    const exp = 1700000000;
    const token = makeJwt({ sub: "user-1", exp });

    expect(extractJwtExpiry(token)).toBe(exp * 1000);
  });

  it("returns null for JWT without exp claim", () => {
    const token = makeJwt({ sub: "user-1" });

    expect(extractJwtExpiry(token)).toBeNull();
  });

  it("returns null for malformed JWT (wrong number of parts)", () => {
    expect(extractJwtExpiry("not-a-jwt")).toBeNull();
    expect(extractJwtExpiry("two.parts")).toBeNull();
  });

  it("returns null for JWT with invalid base64 payload", () => {
    expect(extractJwtExpiry("a.!!!invalid!!!.c")).toBeNull();
  });

  it("returns null for JWT with non-numeric exp", () => {
    const token = makeJwt({ exp: "not-a-number" });

    expect(extractJwtExpiry(token)).toBeNull();
  });

  it("returns null for empty string", () => {
    expect(extractJwtExpiry("")).toBeNull();
  });
});
