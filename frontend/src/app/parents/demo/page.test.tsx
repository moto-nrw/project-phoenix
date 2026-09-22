import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import ParentDemoEntryPage from "./page";

const signIn = vi.fn();
vi.mock("next-auth/react", () => ({
  signIn: (...args: unknown[]) => signIn(...args) as unknown,
}));

const fetchMock = vi.fn();
const assign = vi.fn();

function json(status: number, body: unknown = {}) {
  return Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function open(hash: string) {
  globalThis.history.replaceState(null, "", `/demo${hash}`);
  return render(<ParentDemoEntryPage />);
}

const READY = {
  status: "ready",
  school_name: "OGS Beispiel",
  school_url: "https://ogs-beispiel-k3m9xp.demo.example",
};

beforeEach(() => {
  localStorage.clear();
  signIn.mockReset().mockResolvedValue({ error: undefined });
  fetchMock.mockReset();
  assign.mockReset();
  vi.stubGlobal("fetch", fetchMock);
  vi.stubGlobal("location", {
    ...globalThis.location,
    get hash() {
      return new URL(document.URL).hash;
    },
    pathname: "/demo",
    search: "",
    assign,
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

// The parents app of the public demo (#3468): the same link redeemed as the
// school's parent of the visitor's name.
describe("ParentDemoEntryPage", () => {
  it("redeems the link as the parent, signs in to the parents app and opens its start page", async () => {
    fetchMock.mockReturnValueOnce(json(200, READY)).mockReturnValueOnce(
      json(200, {
        access_token: "parent-access",
        refresh_token: "parent-refresh",
        demo: { access_id: "4711", role: "parent", src: "messe" },
      }),
    );

    open("#token=secret-token&role=parent");

    await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
    expect(signIn).toHaveBeenCalledWith("parent-credentials", {
      redirect: false,
      internalRefresh: true,
      token: "parent-access",
      refreshToken: "parent-refresh",
    });
    const [url, init] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect(url).toBe("/api/demo/access/sessions");
    expect(init.body).toBe(
      JSON.stringify({ token: "secret-token", role: "parent" }),
    );
    expect(document.URL).not.toContain("secret-token");
    // The banner of the parents app shows the OGS name and reports the entry.
    expect(JSON.parse(localStorage.getItem("moto-demo-visit") ?? "")).toEqual({
      accessId: "4711",
      role: "parent",
      src: "messe",
      schoolName: "OGS Beispiel",
      pending: "demo_entered",
    });
  });

  it("reports a switch when the banner of the OGS app sent the visitor", async () => {
    fetchMock.mockReturnValueOnce(json(200, READY)).mockReturnValueOnce(
      json(200, {
        access_token: "parent-access",
        refresh_token: "parent-refresh",
        demo: { access_id: "4711", role: "parent" },
      }),
    );

    open("#token=secret-token&role=parent&switched=1");

    await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
    expect(
      JSON.parse(localStorage.getItem("moto-demo-visit") ?? "").pending,
    ).toBe("demo_role_switched");
  });

  // The standing school is shared and has no parent of its own: the visitor
  // goes on to the OGS app in the role the session really has.
  it("sends the visitor of the standing school on to the OGS app", async () => {
    fetchMock.mockReturnValueOnce(json(200, READY)).mockReturnValueOnce(
      json(200, {
        access_token: "access",
        refresh_token: "refresh",
        demo: { access_id: "4711", role: "all", fixed_role: true },
      }),
    );

    open("#token=secret-token&role=parent");

    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith(
        "https://ogs-beispiel-k3m9xp.demo.example/demo#token=secret-token&role=all",
      ),
    );
    expect(signIn).not.toHaveBeenCalled();
  });

  it("explains an expired link and links to the website", async () => {
    fetchMock.mockReturnValueOnce(json(410));

    open("#token=secret-token&role=parent");

    expect(
      await screen.findByText("Dieser Link funktioniert nicht mehr"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Neuen Link anfordern" }),
    ).toHaveAttribute("href", "https://moto-ogs.de/demo");
    expect(signIn).not.toHaveBeenCalled();
  });
});
