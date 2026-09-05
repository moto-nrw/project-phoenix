import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { CustomizeDashboardModal } from "./customize-dashboard-modal";
import {
  resolveHomeBlocks,
  type HomeBlockContext,
  type HomeLayoutOverrides,
} from "~/lib/home-blocks";

const context: HomeBlockContext = {
  detailed: true,
  openCareGroupMode: false,
  nfcEnabled: true,
  birthdaysEnabled: true,
};

function renderModal(
  overrides: HomeLayoutOverrides = {},
  policies: Record<string, "optional" | "required" | "disabled"> = {},
  handlers: {
    onSave?: (next: HomeLayoutOverrides) => Promise<void>;
    onReset?: () => Promise<void>;
  } = {},
) {
  const resolved = resolveHomeBlocks(context, overrides, policies);
  const onSave = handlers.onSave ?? vi.fn().mockResolvedValue(undefined);
  const onReset = handlers.onReset ?? vi.fn().mockResolvedValue(undefined);

  render(
    <CustomizeDashboardModal
      isOpen
      onClose={vi.fn()}
      adjustable={resolved.adjustable}
      visible={resolved.visible}
      customized={resolved.customized}
      prescribedCount={Object.keys(policies).length}
      onSave={onSave}
      onReset={onReset}
    />,
  );
  return { onSave, onReset };
}

describe("CustomizeDashboardModal", () => {
  it("speichert nur die Abweichungen vom Standard", async () => {
    const { onSave } = renderModal();

    fireEvent.click(screen.getByLabelText(/Geburtstage/));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    // Nur der abgewählte Baustein steht in der Karte — alles andere entspricht
    // dem Standard und darf nicht mitgespeichert werden, sonst friert es die
    // heutige Empfehlung für immer ein.
    expect(onSave).toHaveBeenCalledWith({ "section.birthdays": false });
  });

  it("speichert eine leere Karte, wenn die Person zum Standard zurückkehrt", async () => {
    const { onSave } = renderModal({ "section.birthdays": false });

    fireEvent.click(screen.getByLabelText(/Geburtstage/));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onSave).toHaveBeenCalledWith({}));
  });

  it("bietet Bausteine, die die Schule festgelegt hat, nicht zur Auswahl an", () => {
    renderModal({}, { "section.birthdays": "required" });

    expect(screen.queryByLabelText(/Geburtstage/)).not.toBeInTheDocument();
    // Ohne diesen Hinweis sucht die Person den Fehler bei sich.
    expect(
      screen.getByText(/Ihre Schule hat\s+eine Kachel\s+fest eingestellt/),
    ).toBeInTheDocument();
  });

  it("sperrt 'Zurücksetzen', solange nichts vom Standard abweicht", () => {
    renderModal();

    expect(screen.getByRole("button", { name: "Zurücksetzen" })).toBeDisabled();
  });

  it("setzt auf Wunsch die empfohlene Ansicht wieder her", async () => {
    const { onReset } = renderModal({ "section.birthdays": false });

    const button = screen.getByRole("button", { name: "Zurücksetzen" });
    expect(button).toBeEnabled();
    fireEvent.click(button);

    await waitFor(() => expect(onReset).toHaveBeenCalled());
  });

  it("meldet einen fehlgeschlagenen Speicherversuch, statt still zu schliessen", async () => {
    renderModal(
      {},
      {},
      {
        onSave: vi.fn().mockRejectedValue(new Error("boom")),
      },
    );

    fireEvent.click(screen.getByLabelText(/Geburtstage/));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(/Das Speichern hat nicht geklappt/),
    ).toBeInTheDocument();
  });

  it("sagt es, wenn die Schule alles festgelegt hat", () => {
    const policies = Object.fromEntries(
      resolveHomeBlocks(context, {}, {}).available.map((block) => [
        block.key,
        "required" as const,
      ]),
    );
    renderModal({}, policies);

    expect(
      screen.getByText(/Ihre Schule hat die Startseite fest eingestellt/),
    ).toBeInTheDocument();
  });
});
