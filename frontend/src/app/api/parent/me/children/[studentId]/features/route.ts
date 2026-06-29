import {
  createParentGetHandler,
  parentApiGet,
} from "~/lib/parent/route-wrapper.server";

interface BackendChildFeatures {
  sick_note_enabled: boolean;
  notes_enabled: boolean;
  pickup_change_enabled: boolean;
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
export const GET = createParentGetHandler<BackendChildFeatures>(
  async (_request, token, params) => {
    const studentId = String(params.studentId);
    return parentApiGet<BackendChildFeatures>(
      `/parent/me/children/${encodeURIComponent(studentId)}/features`,
      token,
    );
  },
);
