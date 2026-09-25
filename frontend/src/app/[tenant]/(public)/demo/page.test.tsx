import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DEMO_ROLE_CHOICE_TITLE } from "~/lib/demo-access";
import DemoEntryPage from "./page";

// next/image resolves its src against the stubbed location, which has no href.
vi.mock("next/image", () => ({
  default: (props: Record<string, unknown>) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img {...props} alt={props.alt as string} />
  ),
}));

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
  return render(<DemoEntryPage />);
}

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

describe("DemoEntryPage", () => {
  it("redeems the fragment token by POST in the preselected role, signs in and opens the start page", async () => {
    fetchMock
      .mockReturnValueOnce(json(200, { status: "ready" }))
      .mockReturnValueOnce(
        json(200, {
          access_token: "access",
          refresh_token: "refresh",
          demo: { access_id: "4711", role: "caregiver", src: "messe" },
        }),
      );

    open("#token=secret-token&role=caregiver");

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
    }
    expect(
      fetchMock.mock.calls.map(([url, init]) => [
        url as string,
        (init as RequestInit).body,
      ]),
    ).toEqual([
      ["/api/demo/access/status", JSON.stringify({ token: "secret-token" })],
      [
        "/api/demo/access/sessions",
        JSON.stringify({ token: "secret-token", role: "caregiver" }),
      ],
    ]);
    expect(document.URL).not.toContain("secret-token");
    expect(screen.queryByText(DEMO_ROLE_CHOICE_TITLE)).toBeNull();
    // The banner reports the entry once the start page has loaded.
    expect(JSON.parse(localStorage.getItem("moto-demo-visit") ?? "")).toEqual({
      accessId: "4711",
      role: "caregiver",
      src: "messe",
      pending: "demo_entered",
    });
  });

  it("asks for a role when the link brings none", async () => {
    fetchMock
      .mockReturnValueOnce(json(200, { status: "ready" }))
      .mockReturnValueOnce(
        json(200, {
          access_token: "access",
          refresh_token: "refresh",
          demo: { access_id: "4711", role: "lead" },
        }),
      );

    open("#token=secret-token");

    expect(await screen.findByText(DEMO_ROLE_CHOICE_TITLE)).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const cards = screen.getAllByRole("button");
    expect(cards.map((card) => card.textContent)).toEqual([
      expect.stringContaining("Betreuungskraft"),
      expect.stringContaining("OGS-Leitung"),
      expect.stringContaining("Elternteil"),
      expect.stringContaining("Alle Funktionen"),
    ]);
    fireEvent.click(screen.getByRole("button", { name: /OGS-Leitung/ }));

    await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
    const [, init] = fetchMock.mock.calls[1] as [string, RequestInit];
    expect(init.body).toBe(
      JSON.stringify({ token: "secret-token", role: "lead" }),
    );
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
      ).toHaveAttribute("href", "https://moto-ogs.de/demo");
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

  it("sends the visitor back to the website when the demo school could not be set up", async () => {
    fetchMock.mockReturnValueOnce(json(200, { status: "failed" }));

    open("#token=secret-token");

    expect(
      await screen.findByText(
        "Die Demo konnte nicht vorbereitet werden. Auf unserer Website bekommen Sie sofort einen neuen Link.",
      ),
    ).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(signIn).not.toHaveBeenCalled();
  });

  it("says so when the demo cannot be opened right now", async () => {
    fetchMock.mockReturnValueOnce(json(500));

    open("#token=secret-token&role=all");

    expect(
      await screen.findByText("Das hat leider nicht geklappt"),
    ).toBeInTheDocument();
    expect(assign).not.toHaveBeenCalled();

    fetchMock
      .mockReturnValueOnce(json(200, { status: "ready" }))
      .mockReturnValueOnce(
        json(200, { access_token: "access", refresh_token: "refresh" }),
      );
    fireEvent.click(
      screen.getByRole("button", { name: "Noch einmal versuchen" }),
    );

    await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
  });

  // The role card parent (#3468) opens the parents app with the same link;
  // this page redeems nothing for it.
  it("hands the link on to the parents app when the visitor chooses the role parent", async () => {
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.demo.example");
    fetchMock.mockReturnValueOnce(json(200, { status: "ready" }));

    open("#token=secret-token");

    expect(await screen.findByText(DEMO_ROLE_CHOICE_TITLE)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /Elternteil/ }));

    await waitFor(() =>
      expect(assign).toHaveBeenCalledWith(
        `${globalThis.location.protocol}//eltern.demo.example/demo#token=secret-token&role=parent`,
      ),
    );
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(signIn).not.toHaveBeenCalled();
    vi.unstubAllEnvs();
  });

  // The banner of the parents app sends the visitor back here (#3468); the
  // entry then counts as a role switch.
  it("reports a switch when the banner of the parents app sent the visitor", async () => {
    fetchMock
      .mockReturnValueOnce(json(200, { status: "ready" }))
      .mockReturnValueOnce(
        json(200, {
          access_token: "access",
          refresh_token: "refresh",
          demo: { access_id: "4711", role: "lead" },
        }),
      );

    open("#token=secret-token&role=lead&switched=1");

    await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
    expect(
      JSON.parse(localStorage.getItem("moto-demo-visit") ?? "").pending,
    ).toBe("demo_role_switched");
  });
});
