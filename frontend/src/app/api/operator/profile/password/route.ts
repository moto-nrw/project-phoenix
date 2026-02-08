import type { NextRequest } from "next/server";
import {
  createOperatorPostHandler,
  operatorApiPost,
} from "~/lib/operator/route-wrapper";

interface ModalBody {
  currentPassword?: string;
  newPassword?: string;
  current_password?: string;
  new_password?: string;
}

interface BackendBody {
  current_password: string;
  new_password: string;
}

export const POST = createOperatorPostHandler<null, ModalBody>(
  async (_request: NextRequest, body: ModalBody, token: string) => {
    const backendBody: BackendBody = {
      current_password: body.current_password ?? body.currentPassword ?? "",
      new_password: body.new_password ?? body.newPassword ?? "",
    };
    return await operatorApiPost<null, BackendBody>(
      "/operator/profile/password",
      token,
      backendBody,
    );
  },
);
