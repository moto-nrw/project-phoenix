import type { NextRequest } from "next/server";
import {
  createOperatorGetHandler,
  createOperatorPutHandler,
  operatorApiGet,
  operatorApiPut,
} from "~/lib/operator/route-wrapper";

interface ProfileResponse {
  id: number;
  email: string;
  display_name: string;
}

interface UpdateProfileBody {
  display_name: string;
}

export const GET = createOperatorGetHandler<ProfileResponse>(
  async (_request: NextRequest, token: string) => {
    return await operatorApiGet<ProfileResponse>("/operator/profile", token);
  },
);

export const PUT = createOperatorPutHandler<ProfileResponse, UpdateProfileBody>(
  async (_request: NextRequest, body: UpdateProfileBody, token: string) => {
    return await operatorApiPut<ProfileResponse, UpdateProfileBody>(
      "/operator/profile",
      token,
      body,
    );
  },
);
