export function buildOpenAPIKeyExportText(apiKey: string, baseURL: string): string {
  return [
    "DEEIX Open API Key",
    "",
    `API Key: ${apiKey}`,
    `Base URL: ${baseURL}`,
    "Format: OpenAI Compatible Chat Completions",
    "Endpoint: POST /v1/chat/completions",
    "",
  ].join("\n");
}
