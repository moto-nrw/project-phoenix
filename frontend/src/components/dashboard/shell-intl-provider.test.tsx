import { render } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { ShellIntlProvider } from "./shell-intl-provider";

const intlProvider = vi.hoisted(() => ({
  messages: undefined as unknown,
}));

vi.mock("next-intl", () => ({
  NextIntlClientProvider: ({
    children,
    messages,
  }: {
    children: ReactNode;
    messages: unknown;
  }) => {
    intlProvider.messages = messages;
    return children;
  },
}));

describe("ShellIntlProvider", () => {
  it("stellt die Texte für die Einwilligungen in den Schüler-Stammdaten bereit", () => {
    render(
      <ShellIntlProvider>
        <div>Inhalt</div>
      </ShellIntlProvider>,
    );

    expect(intlProvider.messages).toMatchObject({
      parentChild: {
        consents: {
          title: "Einwilligungen und Bestätigungen",
          staffDescription:
            "Nur zur Information. Eltern ändern die Foto-Einwilligung im Elternportal.",
          items: {
            agb: "Allgemeine Geschäftsbedingungen (AGB)",
            data_processing: "Datenschutz zur Kenntnis genommen",
            photo: "Foto-Einwilligung",
          },
          states: {
            granted: "Hinterlegt",
            withdrawn: "Widerrufen",
          },
          dates: {
            granted: "Hinterlegt am {date}",
            withdrawn: "Widerrufen am {date}",
          },
        },
      },
    });
  });
});
