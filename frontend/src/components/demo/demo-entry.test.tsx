import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DemoEntry } from "./demo-entry";

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
  return render(<DemoEntry />);
}

beforeEach(() => {
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

describe("DemoEntry", () => {
  it("redeems the fragment token by POST, signs in and opens the start page", async () => {
    fetchMock
      .mockReturnValueOnce(json(200, { status: "ready" }))
      .mockReturnValueOnce(
        json(200, { access_token: "access", refresh_token: "refresh" }),
      );

    open("#token=secret-token");

    await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
    expect(signIn).toHaveBeenCalledWith("credentials", {
      redirect: false,
      internalRefresh: true,
      token: "access",
      refreshToken: "refresh",
    });
    for (const [url, init] of fetchMock.mock.calls as [string, RequestInit][]) {
      expect(url).not.toContain("secret-token");
      expect(init.method).toBe("POST");
      expect(init.body).toBe(JSON.stringify({ token: "secret-token" }));
    }
    expect(fetchMock.mock.calls.map(([url]) => url as string)).toEqual([
      "/api/demo/access/status",
      "/api/demo/access/sessions",
    ]);
    expect(document.URL).not.toContain("secret-token");
  });

  it.each([404, 410])(
    "explains a link the backend answers with %i and links to the website",
    async (status) => {
      fetchMock.mockReturnValueOnce(json(status));

      open("#token=old-token");

      expect(
        await screen.findByText("Dieser Link funktioniert nicht mehr"),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("link", { name: "Neuen Link anfordern" }),
      ).toHaveAttribute("href", "https://moto.nrw/demo/");
      expect(signIn).not.toHaveBeenCalled();
    },
  );

  it("treats a missing token like an unknown link", async () => {
    open("");

    expect(
      await screen.findByText("Dieser Link funktioniert nicht mehr"),
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("says so when the demo cannot be opened right now", async () => {
    fetchMock.mockReturnValueOnce(json(500));

    open("#token=secret-token");

    expect(
      await screen.findByText("Das hat leider nicht geklappt"),
    ).toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
  });
});
