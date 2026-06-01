import { authedRequest } from "@/shared/api/authed-client";

export type OpenAPIKeyDTO = {
  exists: boolean;
  apiKey?: string;
  keyPrefix: string;
  status: "active" | "revoked" | string;
  lastUsedAt?: string;
  createdAt?: string;
  updatedAt?: string;
  twoFactorRequired?: boolean;
  exportable?: boolean;
};

export async function getOpenAPIKey(accessToken: string): Promise<OpenAPIKeyDTO> {
  return authedRequest<OpenAPIKeyDTO>("/api/v1/user/openapi-key", { accessToken }, true);
}

export async function createOpenAPIKey(accessToken: string): Promise<OpenAPIKeyDTO> {
  return authedRequest<OpenAPIKeyDTO>(
    "/api/v1/user/openapi-key",
    { method: "POST", accessToken },
    true,
  );
}

export async function regenerateOpenAPIKey(accessToken: string): Promise<OpenAPIKeyDTO> {
  return authedRequest<OpenAPIKeyDTO>(
    "/api/v1/user/openapi-key/regenerate",
    { method: "POST", accessToken },
    true,
  );
}

export async function deleteOpenAPIKey(accessToken: string): Promise<OpenAPIKeyDTO> {
  return authedRequest<OpenAPIKeyDTO>(
    "/api/v1/user/openapi-key",
    { method: "DELETE", accessToken },
    true,
  );
}
