import { proxyGet } from "~/lib/parent/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";

interface BackendChildFeatures {
  sick_note_enabled: boolean;
  notes_enabled: boolean;
  request_submit_enabled: boolean;
  pickup_change_enabled: boolean;
  pickup_manage_allowed: boolean;
  guardian_contact_manage_allowed: boolean;
  related_accounts_invite_enabled: boolean;
  related_accounts_remove_enabled: boolean;
  master_data_edit_enabled: boolean;
  master_data_contact_edit_enabled: boolean;
  master_data_request_enabled: boolean;
}

/**
 * Proxy GET /api/parent/me/children/{studentId}/features → backend.
 * Returns the resolved per-tenant parent-portal feature toggles so the UI can
 * hide/disable actions the backend would reject.
 */
export const GET = proxyGet<BackendChildFeatures>(
  (params) =>
    `/parent/me/children/${requirePathSegmentParam(params, "studentId")}/features`,
);
