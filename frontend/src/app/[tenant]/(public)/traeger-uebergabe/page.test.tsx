import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  TRAEGER_HANDOFF_BACK,
  TRAEGER_HANDOFF_FAILED_TITLE,
  TRAEGER_HANDOFF_OPENING,
} from "~/lib/traeger-handoff";
import TraegerHandoffPage from "./page";

// next/image resolves its src against the stubbed location, which has no href.
vi.mock("next/image", () => ({
  default: (props: Record<string, unknown>) => (
    // eslint-disable-next-line @next/next/no-img-element
    <img {...props} alt={props.alt as string} />
  ),
}));

vi.mock("~/env.client", () => ({
  clientEnv: {
    NEXT_PUBLIC_TENANT_DOMAIN: "moto-app.de",
    NEXT_PUBLIC_TRAEGER_HOSTNAME: "traeger.moto-app.de",
  },
}));

const signIn = vi.fn();
vi.mock("next-auth/react", () => ({
  signIn: (...args: unknown[]) => signIn(...args) as unknown,
}));

const assign = vi.fn();

function open(hash: string) {
  globalThis.history.replaceState(null, "", `/traeger-uebergabe${hash}`);
  return render(<TraegerHandoffPage />);
}

beforeEach(() => {
  signIn.mockReset().mockResolvedValue({ error: undefined });
  assign.mockReset();
  vi.stubGlobal("location", {
    ...globalThis.location,
    get hash() {
      return new URL(document.URL).hash;
    },
    pathname: "/traeger-uebergabe",
    search: "",
    protocol: "https:",
    assign,
  });
});

describe("TraegerHandoffPage (#4654)", () => {
  it("redeems the hand-off token, removes it from the address and opens the start page", async () => {
    open("#token=handoff-token");

    expect(screen.getByText(TRAEGER_HANDOFF_OPENING)).toBeInTheDocument();
    await waitFor(() => expect(assign).toHaveBeenCalledWith("/"));
    expect(signIn).toHaveBeenCalledWith("credentials", {
      redirect: false,
      traegerHandoff: "handoff-token",
    });
    expect(document.URL).not.toContain("handoff-token");
  });

  it("leads back to the Träger portal when the link no longer works", async () => {
    signIn.mockResolvedValue({ error: "CredentialsSignin" });
    open("#token=used-token");

    expect(
      await screen.findByText(TRAEGER_HANDOFF_FAILED_TITLE),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: TRAEGER_HANDOFF_BACK }),
    ).toHaveAttribute("href", "https://traeger.moto-app.de/");
    expect(assign).not.toHaveBeenCalled();
  });

  it("signs nothing in without a token", async () => {
    open("");

    expect(
      await screen.findByText(TRAEGER_HANDOFF_FAILED_TITLE),
    ).toBeInTheDocument();
    expect(signIn).not.toHaveBeenCalled();
  });
});
