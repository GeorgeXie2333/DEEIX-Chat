export const OPEN_API_TEXT_PROTOCOLS = [
  "openai_responses",
  "openai_chat_completions",
  "anthropic_messages",
  "google_generate_content",
  "xai_responses",
] as const;

const OPEN_API_TEXT_PROTOCOL_SET = new Set<string>(OPEN_API_TEXT_PROTOCOLS);

export type OpenAPIModelFilterItem = {
  status: string;
  activeSourceCount: number;
  kindsJSON: string;
  protocolsJSON: string;
};

function hasJSONValue(raw: string, value: string): boolean {
  try {
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed) && parsed.some((item) => String(item).trim() === value);
  } catch {
    return false;
  }
}

function hasAnyJSONValue(raw: string, values: Set<string>): boolean {
  try {
    const parsed = JSON.parse(raw) as unknown;
    return Array.isArray(parsed) && parsed.some((item) => values.has(String(item).trim()));
  } catch {
    return false;
  }
}

export function isOpenAPITextModel(model: OpenAPIModelFilterItem): boolean {
  return (
    model.status === "active" &&
    model.activeSourceCount > 0 &&
    hasJSONValue(model.kindsJSON, "chat") &&
    hasAnyJSONValue(model.protocolsJSON, OPEN_API_TEXT_PROTOCOL_SET)
  );
}
