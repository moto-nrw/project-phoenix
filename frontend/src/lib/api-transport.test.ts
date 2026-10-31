import { CanceledError, type InternalAxiosRequestConfig } from "axios";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import api from "./api-transport";
import {
  beginSessionEnd,
  resetSessionBarrierForTests,
} from "./session-end-barrier";
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

/**
 * #5399: a logout aborts the tab's requests to its own BFF before their
 * answer can write the ending session back, and holds new ones. XHR sets
 * cookies like fetch does.
 */
describe("Axios behind the session-end barrier", () => {
  type Pending = {
    config: InternalAxiosRequestConfig;
    answer: () => void;
  };
  let pending: Pending[] = [];

  beforeEach(() => {
    pending = [];
    resetSessionBarrierForTests();
    api.defaults.adapter = (config) =>
      new Promise((resolve, reject) => {
        const signal = config.signal as AbortSignal | undefined;
        signal?.addEventListener(
          "abort",
          () => reject(new CanceledError(undefined, config)),
          { once: true },
        );
        pending.push({
          config,
          answer: () =>
            resolve({
              data: {},
              status: 200,
              statusText: "OK",
              headers: {},
              config,
            }),
        });
      });
  });
  afterEach(() => resetSessionBarrierForTests());

  const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

  it("asks again for an aborted read once the logout fails", async () => {
    const read = api.get(`${window.location.origin}/api/students`);
    await flush();
    expect(pending).toHaveLength(1);

    const end = beginSessionEnd();
    await flush();
    expect(pending).toHaveLength(1);

    end.fail();
    await vi.waitFor(() => expect(pending).toHaveLength(2));
    pending[1]!.answer();
    await expect(read).resolves.toMatchObject({ status: 200 });
  });

  it("fails an aborted write instead of sending it twice", async () => {
    const write = api.post(`${window.location.origin}/api/students`, {});
    await flush();

    const end = beginSessionEnd();

    await expect(write).rejects.toBeInstanceOf(CanceledError);
    end.fail();
    await flush();
    expect(pending).toHaveLength(1);
  });
});
