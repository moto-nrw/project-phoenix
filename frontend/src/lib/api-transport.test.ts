import type { InternalAxiosRequestConfig } from "axios";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import api from "./api-transport";
import { bindSessionAccount, SESSION_ACCOUNT_HEADER } from "./session-account";

/**
 * #5183: Axios names the tab's account on requests to the tab's own BFF,
 * like the fetch observer. Its base URL is the backend on another origin;
 * that origin never gets the header, it would need a CORS preflight.
 */
let sent: InternalAxiosRequestConfig[] = [];
const originalAdapter = api.defaults.adapter;

beforeEach(() => {
  sent = [];
  api.defaults.adapter = async (config) => {
    sent.push(config);
    return { data: {}, status: 200, statusText: "OK", headers: {}, config };
  };
});

afterEach(() => {
  api.defaults.adapter = originalAdapter;
  bindSessionAccount(null);
});

describe("the tab's account on Axios requests", () => {
  it("names the bound account on a request to this origin's /api/", async () => {
    bindSessionAccount("7");

    await api.post(`${window.location.origin}/api/students`, {});

    expect(sent[0]?.headers.get(SESSION_ACCOUNT_HEADER)).toBe("7");
  });

  it("sends no account to the backend's origin", async () => {
    bindSessionAccount("7");

    await api.get("/api/students");

    expect(api.getUri(sent[0])).toBe("http://localhost:8080/api/students");
    expect(sent[0]?.headers.has(SESSION_ACCOUNT_HEADER)).toBe(false);
  });

  it("names no account while the tab shows none", async () => {
    await api.get(`${window.location.origin}/api/students`);

    expect(sent[0]?.headers.has(SESSION_ACCOUNT_HEADER)).toBe(false);
  });
});
