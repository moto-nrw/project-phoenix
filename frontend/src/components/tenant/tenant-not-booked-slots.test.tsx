import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { FormErrorAlert } from "~/components/ui/form-error-alert";

import { TenantNotBookedSlots } from "./tenant-not-booked-slots";

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

describe("TenantNotBookedSlots", () => {
  it("fills the kit's error alert with the module explanation", async () => {
    render(
      <TenantNotBookedSlots>
        <FormErrorAlert
          message={{
            message: "Katalogtext",
            notBookedModule: "zeiterfassung",
            attempt: 1,
          }}
        />
      </TenantNotBookedSlots>,
    );

    expect(
      await screen.findByText(/Diese Seite gehört zum Modul Zeiterfassung./),
    ).toBeInTheDocument();
    expect(screen.queryByText("Katalogtext")).not.toBeInTheDocument();
  });
});
