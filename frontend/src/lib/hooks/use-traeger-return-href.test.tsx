import { renderToString } from "react-dom/server";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useShellAuth } from "~/lib/shell-auth-context";
import { useTraegerReturnHref } from "./use-traeger-return-href";

vi.mock("~/lib/shell-auth-context", () => ({ useShellAuth: vi.fn() }));
vi.mock("~/env.client", () => ({
  clientEnv: { NEXT_PUBLIC_TRAEGER_HOSTNAME: "traeger.test" },
}));

function Probe() {
  return <span>{useTraegerReturnHref() ?? "kein Rückweg"}</span>;
}

describe("useTraegerReturnHref (#5687)", () => {
  const session = (isViaTraeger: boolean) =>
    vi.mocked(useShellAuth).mockReturnValue({
      isViaTraeger,
    } as ReturnType<typeof useShellAuth>);

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("names the Träger portal for a session opened through the Träger", () => {
    session(true);
    render(<Probe />);

    expect(screen.getByText("http://traeger.test/")).toBeInTheDocument();
  });

  it("names nothing for a session without the Träger origin", () => {
    session(false);
    render(<Probe />);

    expect(screen.getByText("kein Rückweg")).toBeInTheDocument();
  });

  // The tenant layout hydrates the session on the server, where no address
  // of the page exists to build the link from.
  it("names nothing in the server render of such a session", () => {
    session(true);

    expect(renderToString(<Probe />)).toContain("kein Rückweg");
  });
});
