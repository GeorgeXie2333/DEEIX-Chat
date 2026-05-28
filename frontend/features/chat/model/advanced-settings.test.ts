import assert from "node:assert/strict";
import test from "node:test";

import {
  resetAdvancedSettings,
  resolveAdvancedSettings,
  setAdvancedSettingValue,
} from "./advanced-settings.ts";
import type { ModelOptionPolicy } from "../../../shared/lib/model-option-policy.ts";

const allowAdvancedPolicy: ModelOptionPolicy = {
  mode: "allowlist",
  allowedPathsJSON: JSON.stringify({
    default: ["temperature"],
    openai_chat_completions: ["reasoning_effort", "verbosity"],
    openai_responses: ["reasoning.effort", "text.verbosity"],
    xai_responses: ["reasoning.effort"],
  }),
  deniedPathsJSON: "{}",
  nativeToolAllowedTypesJSON: "{}",
};

test("resolveAdvancedSettings maps settings to protocol-specific option paths", () => {
  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "openai_chat_completions",
      options: { temperature: 0.4, reasoning_effort: "high", verbosity: "low" },
      defaultOptions: {},
      policy: allowAdvancedPolicy,
    }).map((item) => [item.kind, item.key, item.value]),
    [
      ["temperature", "temperature", 0.4],
      ["reasoningEffort", "reasoning_effort", "high"],
      ["verbosity", "verbosity", "low"],
    ],
  );

  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "openai_responses",
      options: { temperature: 0.2, reasoning: { effort: "low" }, text: { verbosity: "high" } },
      defaultOptions: {},
      policy: allowAdvancedPolicy,
    }).map((item) => [item.kind, item.key, item.value]),
    [
      ["temperature", "temperature", 0.2],
      ["reasoningEffort", "reasoning.effort", "low"],
      ["verbosity", "text.verbosity", "high"],
    ],
  );
});

test("resolveAdvancedSettings hides fields blocked by the model option policy", () => {
  const policy: ModelOptionPolicy = {
    ...allowAdvancedPolicy,
    allowedPathsJSON: JSON.stringify({ default: ["temperature"] }),
  };

  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "openai_responses",
      options: { temperature: 0.5, reasoning: { effort: "high" }, text: { verbosity: "low" } },
      defaultOptions: {},
      policy,
    }).map((item) => item.key),
    ["temperature"],
  );
});

test("setAdvancedSettingValue writes nested Responses verbosity without disturbing sibling text options", () => {
  const verbosity = resolveAdvancedSettings({
    protocol: "openai_responses",
    options: { text: { format: { type: "json_object" } } },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "verbosity");
  assert.ok(verbosity);

  assert.deepEqual(setAdvancedSettingValue({ text: { format: { type: "json_object" } } }, verbosity, "high"), {
    text: {
      format: { type: "json_object" },
      verbosity: "high",
    },
  });
});

test("resetAdvancedSettings only resets advanced fields and preserves tools and sibling options", () => {
  const result = resetAdvancedSettings(
    {
      temperature: 0.1,
      reasoning: { effort: "high", summary: "auto" },
      text: { verbosity: "high", format: { type: "json_object" } },
      tools: [{ type: "web_search" }],
    },
    {
      temperature: 0.8,
      reasoning: { effort: "low" },
    },
    "openai_responses",
    allowAdvancedPolicy,
  );

  assert.deepEqual(result, {
    temperature: 0.8,
    reasoning: { summary: "auto", effort: "low" },
    text: { format: { type: "json_object" } },
    tools: [{ type: "web_search" }],
  });
});

test("resolveAdvancedSettings exposes xAI reasoning effort but not verbosity", () => {
  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "xai_responses",
      options: { temperature: 0.5, reasoning: { effort: "high" }, text: { verbosity: "low" } },
      defaultOptions: {},
      policy: allowAdvancedPolicy,
    }).map((item) => item.key),
    ["temperature", "reasoning.effort"],
  );
});
