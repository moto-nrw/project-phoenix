/**
 * Freigabe „Personal beim Träger“ im Operator-Portal (#4705, ADR 0089
 * Abschnitt 2). Vor der Freigabe bestätigt der Träger eine von zwei
 * Erklärungen; das moto-Team hält sie mit der Freigabe fest.
 */
import { operatorFetch } from "./api-helpers";

export type StaffRepresentation = "agreement" | "none";

export interface TraegerStaffReleaseState {
  readonly release: {
    readonly representation: StaffRepresentation;
    readonly releasedAt: string;
  } | null;
  /** Wie viele Personalakten der Träger führt. */
  readonly records: number;
}

interface BackendState {
  release: { representation: StaffRepresentation; released_at: string } | null;
  records: number;
}

const base = (orgId: string) =>
  `/api/operator/provisioning/organizations/${encodeURIComponent(orgId)}/staff-release`;

export const traegerStaffReleaseService = {
  async getState(orgId: string): Promise<TraegerStaffReleaseState> {
    const data = await operatorFetch<BackendState>(base(orgId));
    return {
      release: data.release
        ? {
            representation: data.release.representation,
            releasedAt: data.release.released_at,
          }
        : null,
      records: data.records ?? 0,
    };
  },
  async grant(
    orgId: string,
    representation: StaffRepresentation,
  ): Promise<void> {
    await operatorFetch<unknown>(base(orgId), {
      method: "POST",
      body: { representation },
    });
  },
  async withdraw(orgId: string): Promise<void> {
    await operatorFetch<unknown>(base(orgId), { method: "DELETE" });
  },
};
