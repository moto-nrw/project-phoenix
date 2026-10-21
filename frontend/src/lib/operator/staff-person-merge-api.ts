import { operatorFetch } from "./api-helpers";

type BackendSide = {
  person: {
    id: string;
    first_name: string;
    last_name: string;
    account_id?: string;
    created_at: string;
    birthday?: string;
  };
  employment: { id: string; created_at: string; deleted_at: string | null };
};
type BackendCandidate = {
  keeper: BackendSide;
  donor: BackendSide;
  revision: string;
};
export type MergeSide = {
  personId: string;
  name: string;
  accountId?: string;
  createdAt: string;
  birthday?: string;
  staffId: string;
  staffCreatedAt: string;
  retiredAt: string | null;
};
export type StaffMergeCandidate = {
  keeper: MergeSide;
  donor: MergeSide;
  revision: string;
};
function mapSide(side: BackendSide): MergeSide {
  return {
    personId: String(side.person.id),
    name: `${side.person.first_name} ${side.person.last_name}`,
    accountId: side.person.account_id?.toString(),
    createdAt: side.person.created_at,
    birthday: side.person.birthday,
    staffId: String(side.employment.id),
    staffCreatedAt: side.employment.created_at,
    retiredAt: side.employment.deleted_at,
  };
}
export async function previewStaffPersonMerges(
  schoolId: string,
): Promise<StaffMergeCandidate[]> {
  const candidates = await operatorFetch<BackendCandidate[]>(
    `/api/operator/provisioning/staff-person-merges/${encodeURIComponent(schoolId)}`,
  );
  return candidates.map((candidate) => ({
    keeper: mapSide(candidate.keeper),
    donor: mapSide(candidate.donor),
    revision: candidate.revision,
  }));
}
export async function executeStaffPersonMerges(
  schoolId: string,
  candidates: StaffMergeCandidate[],
  schoolConfirmed: boolean,
): Promise<void> {
  await operatorFetch(
    `/api/operator/provisioning/staff-person-merges/${encodeURIComponent(schoolId)}`,
    {
      method: "POST",
      body: {
        school_confirmed: schoolConfirmed,
        cases: candidates.map((candidate) => ({
          donor_id: candidate.donor.personId,
          keeper_id: candidate.keeper.personId,
          revision: candidate.revision,
        })),
      },
    },
  );
}
