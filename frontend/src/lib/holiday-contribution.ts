/**
 * The holiday contribution as the Elternabrechnung computes it (#4664): the
 * lines of the rule chain per child in whole cent. The public registration
 * form and the Träger portal share this shape, so neither imports the other
 * portal's client.
 */

type ContributionLineKind = "base" | "sibling" | "benefits_exemption";

export interface ContributionLine {
  readonly kind: ContributionLineKind;
  /** Negative for a discount. */
  readonly amountCents: number;
  readonly weekStartsOn: string | null;
  readonly weekEndsOn: string | null;
  readonly timeModelName: string;
  readonly childRank: number | null;
  readonly discountBasisPoints: number | null;
  readonly discountCents: number | null;
  readonly note: string;
}

interface ChildContribution {
  /** The child of a registration; empty in a quote. */
  readonly childId: string;
  /** Place of the child in the form, from 1. */
  readonly position: number;
  readonly lines: readonly ContributionLine[];
  readonly totalCents: number;
}

export interface HolidayContribution {
  readonly children: readonly ChildContribution[];
  readonly totalCents: number;
  /** The Träger has a valid exemption rule. */
  readonly exemptionAvailable: boolean;
  /** The exemption applies. */
  readonly benefitsExemption: boolean;
  /** Frozen at the confirmation; a preview otherwise. */
  readonly frozen: boolean;
  readonly confirmedAt: string | null;
}

interface BackendLine {
  kind: ContributionLineKind;
  amount_cents: number;
  week_starts_on: string | null;
  week_ends_on: string | null;
  time_model_name: string;
  child_rank: number | null;
  discount_basis_points: number | null;
  discount_cents: number | null;
  note: string;
}

export interface BackendContribution {
  children?:
    | {
        child_id: number;
        position: number;
        lines?: BackendLine[] | null;
        total_cents: number;
      }[]
    | null;
  total_cents: number;
  exemption_available: boolean;
  benefits_exemption: boolean;
  frozen: boolean;
  confirmed_at: string | null;
}

/** Maps a contribution of the backend. */
export function contributionFromBackend(
  contribution: BackendContribution,
): HolidayContribution {
  return {
    children: (contribution.children ?? []).map((child) => ({
      childId: child.child_id ? String(child.child_id) : "",
      position: child.position,
      totalCents: child.total_cents,
      lines: (child.lines ?? []).map((line) => ({
        kind: line.kind,
        amountCents: line.amount_cents,
        weekStartsOn: line.week_starts_on,
        weekEndsOn: line.week_ends_on,
        timeModelName: line.time_model_name,
        childRank: line.child_rank,
        discountBasisPoints: line.discount_basis_points,
        discountCents: line.discount_cents,
        note: line.note,
      })),
    })),
    totalCents: contribution.total_cents,
    exemptionAvailable: contribution.exemption_available,
    benefitsExemption: contribution.benefits_exemption,
    frozen: contribution.frozen,
    confirmedAt: contribution.confirmed_at,
  };
}
