import { act, createElement } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// The school, caterer, operator and Träger layouts render the signed-in
// portal on the server (#4991). Links built from the portal path helpers must
// come out the same on the server and during hydration on the portal host;
// a mismatch made React throw away the operator shell (error #418).

const HOST = "portal.localhost:3000";

interface Helper {
  readonly name: string;
  readonly env: string;
  readonly load: () => Promise<(path: string) => string>;
  readonly input: string;
  readonly clean: string;
}

const HELPERS: readonly Helper[] = [
  {
    name: "operatorPath",
    env: "NEXT_PUBLIC_OPERATOR_HOSTNAME",
    load: async () => (await import("./operator-url")).operatorPath,
    input: "/operator/organizations",
    clean: "/organizations",
  },
  {
    name: "schoolPath",
    env: "NEXT_PUBLIC_SCHOOL_HOSTNAME",
    load: async () => (await import("./school-url")).schoolPath,
    input: "/school/kalender",
    clean: "/kalender",
  },
  {
    name: "traegerPath",
    env: "NEXT_PUBLIC_TRAEGER_HOSTNAME",
    load: async () => (await import("./traeger-url")).traegerPath,
    input: "/traeger/accounts",
    clean: "/accounts",
  },
  {
    name: "catererPath",
    env: "NEXT_PUBLIC_CATERER_HOSTNAME",
    load: async () => (await import("./caterer-url")).catererPath,
    input: "/caterer/login",
    clean: "/login",
  },
];

const actualLocation = window.location;

beforeEach(() => {
  vi.resetModules();
  Object.defineProperty(window, "location", {
    writable: true,
    value: { host: HOST, protocol: "http:", origin: `http://${HOST}` },
  });
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
  Object.defineProperty(window, "location", {
    writable: true,
    value: actualLocation,
  });
});

describe.each(HELPERS)("$name", (helper) => {
  it("renders the same href on the server and during hydration on its host", async () => {
    vi.stubEnv(helper.env, HOST);
    const portalPath = await helper.load();
    const Link = () =>
      createElement("a", { href: portalPath(helper.input) }, "Seite");

    const actualWindow = window;
    vi.stubGlobal("window", undefined);
    const serverHTML = renderToString(createElement(Link));
    vi.stubGlobal("window", actualWindow);

    const container = document.createElement("div");
    container.innerHTML = serverHTML;
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    let root: ReturnType<typeof hydrateRoot> | undefined;
    await act(async () => {
      root = hydrateRoot(container, createElement(Link));
    });

    expect(container.querySelector("a")).toHaveAttribute("href", helper.clean);
    expect(consoleError.mock.calls.flat().join(" ")).not.toContain(
      "hydrated but some attributes",
    );

    await act(async () => root?.unmount());
  });

  it("keeps the portal prefix in the browser on another host", async () => {
    vi.stubEnv(helper.env, "elsewhere.localhost:3000");
    const portalPath = await helper.load();

    expect(portalPath(helper.input)).toBe(helper.input);
  });
});
