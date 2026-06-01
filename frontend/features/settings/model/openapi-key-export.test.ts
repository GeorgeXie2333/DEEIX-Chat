import assert from "node:assert/strict";
import test from "node:test";

import { buildOpenAPIKeyExportText } from "./openapi-key-export.ts";

test("buildOpenAPIKeyExportText includes key, base URL, format and chat completions endpoint", () => {
  const text = buildOpenAPIKeyExportText("dxsk_test", "https://api.example.com/v1");

  assert.match(text, /API Key: dxsk_test/);
  assert.match(text, /Base URL: https:\/\/api\.example\.com\/v1/);
  assert.match(text, /Format: OpenAI Compatible Chat Completions/);
  assert.match(text, /Endpoint: POST \/v1\/chat\/completions/);
});
