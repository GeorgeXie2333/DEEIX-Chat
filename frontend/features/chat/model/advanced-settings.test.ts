import assert from "node:assert/strict";
import test from "node:test";

import {
  isGemini3PlusModel,
  isValidOpenAIImage2Resolution,
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
    anthropic_messages: ["output_config.effort"],
    gemini_generate_content: ["thinkingConfig.thinkingLevel"],
    openai_image_generations: ["quality", "size"],
    openai_image_edits: ["quality", "size"],
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

test("resolveAdvancedSettings exposes updated reasoning effort values by provider", () => {
  const openAIReasoning = resolveAdvancedSettings({
    protocol: "openai_responses",
    options: { reasoning: { effort: "xhigh" } },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "reasoningEffort");

  assert.equal(openAIReasoning?.value, "xhigh");
  assert.deepEqual(openAIReasoning?.values, ["none", "low", "medium", "high", "xhigh"]);

  const xAIReasoning = resolveAdvancedSettings({
    protocol: "xai_responses",
    options: { reasoning: { effort: "none" } },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "reasoningEffort");

  assert.equal(xAIReasoning?.value, "none");
  assert.deepEqual(xAIReasoning?.values, ["none", "low", "medium", "high"]);
});

test("resolveAdvancedSettings maps Anthropic effort with medium default", () => {
  const effort = resolveAdvancedSettings({
    protocol: "anthropic_messages",
    options: {},
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "reasoningEffort");
  assert.ok(effort);
  assert.equal(effort.key, "output_config.effort");
  assert.equal(effort.value, "medium");

  assert.deepEqual(setAdvancedSettingValue({ output_config: { format: { type: "json" } } }, effort, "max"), {
    output_config: {
      format: { type: "json" },
      effort: "max",
    },
  });
});

test("resolveAdvancedSettings only exposes Gemini thinking level for Gemini 3+", () => {
  assert.equal(isGemini3PlusModel("gemini-3-pro-preview"), true);
  assert.equal(isGemini3PlusModel("google/gemini_3.1_flash"), true);
  assert.equal(isGemini3PlusModel("gemini-2.5-pro"), false);

  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "gemini_generate_content",
      modelName: "gemini-3-pro-preview",
      options: { thinkingConfig: { thinkingLevel: "high" } },
      defaultOptions: {},
      policy: allowAdvancedPolicy,
    }).map((item) => [item.kind, item.key, item.value]),
    [
      ["temperature", "temperature", 1],
      ["reasoningEffort", "thinkingConfig.thinkingLevel", "high"],
    ],
  );

  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "gemini_generate_content",
      modelName: "gemini-2.5-pro",
      options: { thinkingConfig: { thinkingLevel: "high" } },
      defaultOptions: {},
      policy: allowAdvancedPolicy,
    }),
    [],
  );
});

test("resolveAdvancedSettings exposes OpenAI image quality and gpt-image-2 custom resolution", () => {
  const settings = resolveAdvancedSettings({
    protocol: "openai_image_generations",
    modelName: "gpt-image-2",
    options: { quality: "high", size: "2048x1152" },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  });
  const quality = settings.find((item) => item.kind === "imageQuality");
  const resolution = settings.find((item) => item.kind === "imageResolution");

  assert.equal(quality?.key, "quality");
  assert.equal(quality?.value, "high");
  assert.equal(resolution?.key, "size");
  assert.equal(resolution?.value, "2048x1152");
  assert.deepEqual(resolution?.values, [
    "auto",
    "1024x1024",
    "1536x1024",
    "1024x1536",
    "2048x2048",
    "2048x1152",
    "3840x2160",
    "2160x3840",
  ]);
  assert.equal(isValidOpenAIImage2Resolution("2048x1152"), true);
  assert.equal(isValidOpenAIImage2Resolution("2049x1152"), false);
  assert.equal(isValidOpenAIImage2Resolution("4096x1024"), false);

  assert.deepEqual(setAdvancedSettingValue({}, resolution!, "1600x1024"), { size: "1600x1024" });
  assert.deepEqual(setAdvancedSettingValue({ size: "1024x1024" }, resolution!, "1000x1000"), {
    size: "1024x1024",
  });
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

test("resolveAdvancedSettings upgrades legacy default policy paths for Claude and Gemini", () => {
  const legacyDefaultPolicy: ModelOptionPolicy = {
    ...allowAdvancedPolicy,
    allowedPathsJSON: JSON.stringify({
      default: ["temperature"],
      anthropic_messages: ["speed", "top_k", "thinking.type"],
      gemini_generate_content: [
        "generationConfig.temperature",
        "generationConfig.topP",
        "generationConfig.maxOutputTokens",
        "generationConfig.responseMimeType",
      ],
    }),
  };

  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "anthropic_messages",
      options: {},
      defaultOptions: {},
      policy: legacyDefaultPolicy,
    }).map((item) => item.key),
    ["temperature", "output_config.effort"],
  );
  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "gemini_generate_content",
      modelName: "gemini-3-pro-preview",
      options: {},
      defaultOptions: {},
      policy: legacyDefaultPolicy,
    }).map((item) => item.key),
    ["temperature", "thinkingConfig.thinkingLevel"],
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
