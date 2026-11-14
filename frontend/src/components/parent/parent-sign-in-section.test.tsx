import { fireEvent, render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";

import deMessages from "~/i18n/messages/de.generated";
import { ToastProvider } from "~/contexts/ToastContext";

const factor = vi.hoisted(() => ({
  status: vi.fn(),
  start: vi.fn(),
  confirm: vi.fn(),
  disable: vi.fn(),
}));

vi.mock("~/lib/email-code-factor-api", () => ({
  emailCodeFactorApi: () => factor,
}));
vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ logout: vi.fn() }),
}));
// The device card has tests of its own; here it shows whether it is there.
vi.mock("~/components/settings/trusted-devices-section", () => ({
  TrustedDevicesSection: ({ copy }: { copy: { title: string } }) => (
    <p>{copy.title}</p>
  ),
}));

import { ParentSignInSection } from "./parent-sign-in-section";

function renderSection() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <ToastProvider>
        <ParentSignInSection />
      </ToastProvider>
    </NextIntlClientProvider>,
  );
}

// „Einstellungen → Anmeldung“ of the parents portal (#7051). Every language
// carries the same keys (verify-locales); the test reads the German copy.
describe("ParentSignInSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("offers the code by e-mail and explains it while it is off", async () => {
    factor.status.mockResolvedValue({
      enabled: false,
      available: true,
      maskedEmail: "p***@example.com",
    });
    factor.start.mockResolvedValue({
      challengeToken: "challenge",
      maskedEmail: "p***@example.com",
    });
    factor.confirm.mockResolvedValue({ sessionRenewed: true });

    renderSection();

    expect(await screen.findByText("Ausgeschaltet")).toBeVisible();
    expect(screen.getByText("So funktioniert es")).toBeVisible();
    expect(
      screen.getByText(/Wenden Sie sich an Ihre Schule\. Das moto-Team/),
    ).toBeVisible();
    expect(screen.queryByText("Gemerkte Geräte")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Einschalten" }));
    expect(
      await screen.findByText(
        "Wir haben einen Code an p***@example.com geschickt. Geben Sie die 6 Ziffern ein.",
      ),
    ).toBeVisible();
    fireEvent.change(screen.getByLabelText("Code aus der E-Mail"), {
      target: { value: "12a3456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Einschalten" }));

    expect(await screen.findByText("Eingeschaltet")).toBeVisible();
    expect(factor.confirm).toHaveBeenCalledWith("challenge", "123456");
    expect(screen.getByText("Gemerkte Geräte")).toBeVisible();
    expect(screen.getByText("p***@example.com")).toBeVisible();
  });

  it("states why an account without a school cannot switch it on", async () => {
    factor.status.mockResolvedValue({
      enabled: false,
      available: false,
      maskedEmail: "p***@example.com",
    });

    renderSection();

    expect(
      await screen.findByText(
        "Die Anmeldung mit Code können Sie einschalten, sobald Ihr Konto zu einer Schule gehört.",
      ),
    ).toBeVisible();
    expect(
      screen.queryByRole("button", { name: "Einschalten" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("So funktioniert es")).not.toBeInTheDocument();
  });

  it("switches it off with the password", async () => {
    factor.status.mockResolvedValue({
      enabled: true,
      available: true,
      maskedEmail: "p***@example.com",
    });
    factor.disable.mockResolvedValue(undefined);

    renderSection();

    fireEvent.click(await screen.findByRole("button", { name: "Ausschalten" }));
    fireEvent.change(screen.getByLabelText("Ihr Passwort"), {
      target: { value: "secret" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Ausschalten" }));

    expect(await screen.findByText("Ausgeschaltet")).toBeVisible();
    expect(factor.disable).toHaveBeenCalledWith("secret");
    expect(screen.queryByText("Gemerkte Geräte")).not.toBeInTheDocument();
  });
});
