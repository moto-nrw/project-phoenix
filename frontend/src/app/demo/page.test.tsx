import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DemoWaitingRoomPage from "./page";

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
  return render(<DemoWaitingRoomPage />);
}

beforeEach(() => {
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

describe("DemoWaitingRoomPage", () => {
  it("hands the token on to the demo school once it is ready", async () => {
    fetchMock.mockReturnValueOnce(
      json(200, {
        status: "ready",
        school_url: "https://ogs-nord-k3m9xp.demo.example",
      }),
    );

    open("#token=secret-token");

    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith(
        "https://ogs-nord-k3m9xp.demo.example/demo#token=secret-token",
      ),
    );
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/api/demo/access/status");
    expect(url).not.toContain("secret-token");
    expect(init.body).toBe(JSON.stringify({ token: "secret-token" }));
    expect(document.URL).not.toContain("secret-token");
  });

  it("tells the visitor that the demo is being prepared and does not leave", async () => {
    fetchMock.mockReturnValue(json(200, { status: "preparing" }));

    const view = open("#token=secret-token");

    expect(
      await screen.findByText(
        "Ihre Demo wird vorbereitet. Einen Moment bitte.",
      ),
    ).toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
    view.unmount();
  });

  it("sends the visitor back to the website when the demo school could not be set up", async () => {
    fetchMock.mockReturnValueOnce(json(200, { status: "failed" }));

    open("#token=secret-token");

    expect(
      await screen.findByText(
        "Es liegt nicht an Ihnen. Bitte fordern Sie auf unserer Website einen neuen Link an.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Neuen Link anfordern" }),
    ).toHaveAttribute("href", "https://moto-ogs.de/demo");
    expect(assign).not.toHaveBeenCalled();
  });

  it.each([404, 410])(
    "explains a link the backend answers with %i",
    async (status) => {
      fetchMock.mockReturnValueOnce(json(status));

      open("#token=old-token");

      expect(
        await screen.findByText("Dieser Link funktioniert nicht mehr"),
      ).toBeInTheDocument();
    },
  );

  it("does not follow a ready answer without an address", async () => {
    fetchMock.mockReturnValueOnce(json(200, { status: "ready" }));

    open("#token=secret-token");

    expect(
      await screen.findByText("Das hat leider nicht geklappt"),
    ).toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();
  });
});
