import assert from "node:assert/strict";
import test from "node:test";

import { OPEN_API_TEXT_PROTOCOLS, isOpenAPITextModel } from "./openapi-model-filter.ts";

type FilterModel = Parameters<typeof isOpenAPITextModel>[0] & { name: string };

function model(
  name: string,
  protocols: string[],
  overrides: Partial<Parameters<typeof isOpenAPITextModel>[0]> = {},
): FilterModel {
  return {
    name,
    status: "active",
    activeSourceCount: 1,
    kindsJSON: JSON.stringify(["chat"]),
    protocolsJSON: JSON.stringify(protocols),
    ...overrides,
  };
}

test("isOpenAPITextModel accepts all public chat protocols", () => {
  assert.deepEqual(OPEN_API_TEXT_PROTOCOLS, [
    "openai_responses",
    "openai_chat_completions",
    "anthropic_messages",
    "google_generate_content",
    "xai_responses",
  ]);

  const models = [
    model("responses", ["openai_responses"]),
    model("chat", ["openai_chat_completions"]),
    model("anthropic", ["anthropic_messages"]),
    model("google", ["google_generate_content"]),
    model("xai", ["xai_responses"]),
    model("image", ["openai_image_generations"]),
    model("inactive", ["openai_responses"], { status: "inactive" }),
    model("no-source", ["openai_responses"], { activeSourceCount: 0 }),
    model("not-chat", ["openai_responses"], { kindsJSON: JSON.stringify(["image"]) }),
  ];

  assert.deepEqual(models.filter(isOpenAPITextModel).map((item) => item.name), [
    "responses",
    "chat",
    "anthropic",
    "google",
    "xai",
  ]);
});
