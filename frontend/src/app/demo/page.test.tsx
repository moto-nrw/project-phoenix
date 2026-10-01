import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DemoWaitingRoomPage from "./page";

// next/image resolves its src against the stubbed location, which has no href.
vi.mock("next/image", () => ({
  default: (props: Record<string, unknown>) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img {...props} alt={props.alt as string} />
  ),
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
  return render(<DemoWaitingRoomPage />);
}

beforeEach(() => {
  sessionStorage.clear();
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

  it("hands a preselected role on with the token", async () => {
    fetchMock.mockReturnValueOnce(
      json(200, {
        status: "ready",
        school_url: "https://ogs-nord-k3m9xp.demo.example",
      }),
    );

    open("#token=secret-token&role=lead");

    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith(
        "https://ogs-nord-k3m9xp.demo.example/demo#token=secret-token&role=lead",
      ),
    );
  });

  it("names the OGS being set up, shows the progress lines and does not leave", async () => {
    fetchMock.mockReturnValue(
      json(200, { status: "preparing", school_name: "OGS Nord" }),
    );

    const view = open("#token=secret-token");

    expect(
      await screen.findByRole("heading", {
        name: "Wir richten OGS Nord für Sie ein",
      }),
    ).toBeInTheDocument();
    const lines = within(screen.getByRole("list")).getAllByRole("listitem");
    expect(lines.map((line) => line.textContent)).toEqual([
      "Schule anlegenläuft",
      "Kinder und Gruppen eintragenfolgt",
      "OGS-Tag startenfolgt",
    ]);
    // Screen readers hear the line that is running now.
    expect(screen.getByRole("status")).toHaveTextContent("Schule anlegen …");
    expect(assign).not.toHaveBeenCalled();
    view.unmount();
  });

  // The fragment leaves the address bar; a reload during the setup must not
  // turn a valid link into „Dieser Link funktioniert nicht mehr".
  it("keeps waiting for the school after a reload", async () => {
    fetchMock.mockReturnValue(
      json(200, { status: "preparing", school_name: "OGS Nord" }),
    );
    const first = open("#token=secret-token&role=lead");
    expect(
      await screen.findByRole("heading", {
        name: "Wir richten OGS Nord für Sie ein",
      }),
    ).toBeInTheDocument();
    expect(document.URL).not.toContain("secret-token");
    first.unmount();

    fetchMock.mockReset();
    fetchMock.mockReturnValueOnce(
      json(200, {
        status: "ready",
        school_url: "https://ogs-nord-k3m9xp.demo.example",
      }),
    );
    open("");

    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith(
        "https://ogs-nord-k3m9xp.demo.example/demo#token=secret-token&role=lead",
      ),
    );
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(init.body).toBe(JSON.stringify({ token: "secret-token" }));
    expect(
      screen.queryByText("Dieser Link funktioniert nicht mehr"),
    ).toBeNull();
  });

  it("explains a missing link when the tab has none kept", async () => {
    open("");

    expect(
      await screen.findByText("Dieser Link funktioniert nicht mehr"),
    ).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("offers another try when waiting went wrong and enters the school on it", async () => {
    fetchMock.mockRejectedValueOnce(new Error("network down"));

    open("#token=secret-token");

    const retry = await screen.findByRole("button", {
      name: "Noch einmal versuchen",
    });
    expect(
      screen.getByText("Das hat leider nicht geklappt"),
    ).toBeInTheDocument();
    fetchMock.mockReturnValueOnce(
      json(200, {
        status: "ready",
        school_url: "https://ogs-nord-k3m9xp.demo.example",
      }),
    );
    fireEvent.click(retry);

    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith(
        "https://ogs-nord-k3m9xp.demo.example/demo#token=secret-token",
      ),
    );
  });

  it("sends the visitor back to the website when the demo school could not be set up", async () => {
    fetchMock.mockReturnValueOnce(json(200, { status: "failed" }));

    open("#token=secret-token");

    expect(
      await screen.findByText(
        "Die Demo konnte nicht vorbereitet werden. Auf unserer Website bekommen Sie sofort einen neuen Link.",
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

  // The role parent (#3468) goes straight to the parents app, which redeems
  // the same link on its own host.
  it("hands a link with the role parent on to the parents app", async () => {
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.demo.example");
    fetchMock.mockReturnValueOnce(
      json(200, {
        status: "ready",
        school_url: "https://ogs-nord-k3m9xp.demo.example",
      }),
    );

    open("#token=secret-token&role=parent");

    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith(
        `${globalThis.location.protocol}//eltern.demo.example/demo#token=secret-token&role=parent`,
      ),
    );
    vi.unstubAllEnvs();
  });
});
