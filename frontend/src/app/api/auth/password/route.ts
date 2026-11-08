import { apiPost, problemResponse } from "~/lib/api-helpers.server";
import { tenant } from "~/lib/route-wrapper.server";

interface PasswordChangeBody {
  currentPassword?: string;
  newPassword?: string;
  confirmPassword?: string;
}

export const POST = tenant.post<unknown, PasswordChangeBody>(
  async (request, body, token) => {
    const { currentPassword, newPassword, confirmPassword } = body;
    if (!currentPassword || !newPassword || !confirmPassword) {
      return problemResponse(400, "all password fields are required", request);
    }
    if (newPassword !== confirmPassword) {
      return problemResponse(400, "new passwords do not match", request);
    }
    await apiPost("/auth/password", token, {
      current_password: currentPassword,
      new_password: newPassword,
      confirm_password: confirmPassword,
    });
    return { success: true };
  },
  { retry: { off: "a renewed session would send the current password twice" } },
);
