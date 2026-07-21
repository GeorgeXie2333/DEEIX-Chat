import assert from "node:assert/strict";
import test from "node:test";
import {
  type ModelOptionPolicy,
  normalizeModelOptionAllowedPathsJSON,
} from "../../../shared/lib/model-option-policy.ts";
import { DEFAULT_MODEL_OPTION_ALLOWED_PATHS } from "../../admin/model/conversation-settings.ts";
import {
  isGemini3PlusModel,
  isValidOpenAIImage2Resolution,
  resetAdvancedSettings,
  resolveAdvancedSettings,
  setAdvancedSettingValue,
} from "./advanced-settings.ts";

const allowAdvancedPolicy: ModelOptionPolicy = {
  mode: "allowlist",
  allowedPathsJSON: JSON.stringify({
    default: ["temperature"],
    openai_chat_completions: ["reasoning_effort", "reasoning_summary", "verbosity"],
    openrouter_chat_completions: ["reasoning.effort", "reasoning.summary", "verbosity"],
    openai_responses: ["reasoning.mode", "reasoning.effort", "reasoning.summary", "text.verbosity"],
    openrouter_responses: ["reasoning.effort"],
    xai_responses: ["reasoning.effort"],
    anthropic_messages: ["output_config.effort", "speed"],
    gemini_generate_content: ["thinkingConfig.thinkingLevel"],
    openai_image_generations: ["quality", "size", "output_format"],
    openai_image_edits: ["quality", "size"],
    openai_video_generations: ["size", "seconds"],
    google_image_generation: [
      "generationConfig.imageConfig.imageSize",
      "generationConfig.imageConfig.aspectRatio",
      "generationConfig.thinkingConfig.thinkingLevel",
    ],
    xai_image: ["aspect_ratio", "resolution", "response_format"],
    xai_image_edits: ["aspect_ratio", "resolution", "response_format"],
    gemini_interactions: [
      "generation_config.thinking_level",
      "response_format.image_size",
      "generation_config.video_config.task",
    ],
  }),
  deniedPathsJSON: "{}",
  nativeToolAllowedTypesJSON: "{}",
};

const preOpenRouterModelOptionAllowedPaths = {
  anthropic_messages: [
    "speed",
    "top_k",
    "thinking.type",
    "thinking.budget_tokens",
    "output_config.effort",
  ],
  default: [
    "temperature",
    "top_p",
    "max_tokens",
    "max_output_tokens",
    "max_completion_tokens",
    "stop",
    "response_format.type",
  ],
  gemini_generate_content: [
    "generationConfig.temperature",
    "generationConfig.topP",
    "generationConfig.maxOutputTokens",
    "generationConfig.responseMimeType",
    "thinkingConfig.includeThoughts",
    "thinkingConfig.thinkingLevel",
  ],
  google_image_generation: [
    "aspect_ratio",
    "aspectRatio",
    "image_size",
    "imageSize",
    "imageConfig.aspectRatio",
    "imageConfig.imageSize",
    "responseFormat.image.aspectRatio",
    "responseFormat.image.imageSize",
    "generationConfig.imageConfig.aspectRatio",
    "generationConfig.imageConfig.imageSize",
    "generationConfig.responseFormat.image.aspectRatio",
    "generationConfig.responseFormat.image.imageSize",
    "generationConfig.thinkingConfig.thinkingLevel",
  ],
  openai_chat_completions: [
    "service_tier",
    "presence_penalty",
    "frequency_penalty",
    "reasoning_effort",
    "verbosity",
    "thinking.type",
    "stream_options.include_usage",
    "reasoning_summary",
  ],
  openai_image_edits: [
    "background",
    "input_fidelity",
    "n",
    "output_compression",
    "output_format",
    "partial_images",
    "quality",
    "response_format",
    "size",
    "user",
  ],
  openai_image_generations: [
    "background",
    "moderation",
    "n",
    "output_compression",
    "output_format",
    "partial_images",
    "quality",
    "response_format",
    "size",
    "style",
    "user",
  ],
  openai_responses: [
    "service_tier",
    "reasoning.effort",
    "reasoning.summary",
    "text.verbosity",
    "reasoning.mode",
  ],
  openai_video_generations: ["seconds", "size"],
  xai_image: ["aspect_ratio", "n", "resolution", "response_format"],
  xai_image_edits: ["aspect_ratio", "n", "resolution", "response_format"],
  xai_responses: ["reasoning.effort"],
} satisfies Record<string, string[]>;

test("resolveAdvancedSettings maps settings to protocol-specific option paths", () => {
  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "openai_chat_completions",
      options: { temperature: 0.4, reasoning_effort: "high", reasoning_summary: "concise", verbosity: "low" },
      defaultOptions: {},
      policy: allowAdvancedPolicy,
    }).map((item) => [item.kind, item.key, item.value]),
    [
      ["temperature", "temperature", 0.4],
      ["reasoningEffort", "reasoning_effort", "high"],
      ["reasoningSummary", "reasoning_summary", "concise"],
      ["verbosity", "verbosity", "low"],
    ],
  );

  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "openai_responses",
      options: { temperature: 0.2, reasoning: { effort: "low", summary: "auto" }, text: { verbosity: "high" } },
      defaultOptions: {},
      policy: allowAdvancedPolicy,
    }).map((item) => [item.kind, item.key, item.value]),
    [
      ["temperature", "temperature", 0.2],
      ["reasoningMode", "reasoning.mode", "standard"],
      ["reasoningEffort", "reasoning.effort", "low"],
      ["reasoningSummary", "reasoning.summary", "auto"],
      ["verbosity", "text.verbosity", "high"],
    ],
  );
});

test("Chat advanced controls default to none and omit none values", () => {
  const openAISettings = resolveAdvancedSettings({
    protocol: "openai_chat_completions",
    options: {},
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  });
  const effort = openAISettings.find((item) => item.kind === "reasoningEffort");
  const summary = openAISettings.find((item) => item.kind === "reasoningSummary");
  const verbosity = openAISettings.find((item) => item.kind === "verbosity");
  assert.ok(effort);
  assert.ok(summary);
  assert.ok(verbosity);
  assert.equal(effort.value, "none");
  assert.deepEqual(effort.values, ["none", "low", "medium", "high", "xhigh", "max"]);
  assert.equal(summary.value, "none");
  assert.deepEqual(summary.values, ["none", "auto", "concise", "detailed"]);
  assert.equal(verbosity.value, "none");
  assert.deepEqual(verbosity.values, ["none", "low", "medium", "high"]);

  const configured = setAdvancedSettingValue(
    setAdvancedSettingValue(setAdvancedSettingValue({ metadata: { tenant: "deeix" } }, effort, "high"), summary, "auto"),
    verbosity,
    "low",
  );
  assert.deepEqual(configured, {
    metadata: { tenant: "deeix" },
    reasoning_effort: "high",
    reasoning_summary: "auto",
    verbosity: "low",
  });
  assert.deepEqual(
    setAdvancedSettingValue(
      setAdvancedSettingValue(setAdvancedSettingValue(configured, effort, "none"), summary, "none"),
      verbosity,
      "none",
    ),
    { metadata: { tenant: "deeix" } },
  );
});

test("OpenRouter Chat advanced controls use nested reasoning paths and prune empty parents", () => {
  const settings = resolveAdvancedSettings({
    protocol: "openrouter_chat_completions",
    options: { reasoning: { effort: "xhigh", summary: "detailed" }, verbosity: "high" },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  });
  assert.deepEqual(
    settings.map((item) => [item.kind, item.key, item.value]),
    [
      ["temperature", "temperature", 1],
      ["reasoningEffort", "reasoning.effort", "xhigh"],
      ["reasoningSummary", "reasoning.summary", "detailed"],
      ["verbosity", "verbosity", "high"],
    ],
  );

  const effort = settings.find((item) => item.kind === "reasoningEffort");
  const summary = settings.find((item) => item.kind === "reasoningSummary");
  assert.ok(effort);
  assert.ok(summary);
  const withoutEffort = setAdvancedSettingValue({ reasoning: { effort: "high", summary: "auto" } }, effort, "none");
  assert.deepEqual(withoutEffort, { reasoning: { summary: "auto" } });
  assert.deepEqual(setAdvancedSettingValue(withoutEffort, summary, "none"), {});
});

test("resolveAdvancedSettings exposes updated reasoning effort values by provider", () => {
  const openAIReasoning = resolveAdvancedSettings({
    protocol: "openai_responses",
    options: { reasoning: { effort: "xhigh" } },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "reasoningEffort");

  assert.equal(openAIReasoning?.value, "xhigh");
  assert.deepEqual(openAIReasoning?.values, ["none", "low", "medium", "high", "xhigh", "max"]);

  const xAIReasoning = resolveAdvancedSettings({
    protocol: "xai_responses",
    options: { reasoning: { effort: "none" } },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "reasoningEffort");

  assert.equal(xAIReasoning?.value, "none");
  assert.deepEqual(xAIReasoning?.values, ["none", "low", "medium", "high"]);
});

test("OpenAI Responses reasoning mode writes pro and omits standard without disturbing sibling options", () => {
  const mode = resolveAdvancedSettings({
    protocol: "openai_responses",
    options: { reasoning: { effort: "max", summary: "auto" } },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "reasoningMode");
  assert.ok(mode);

  assert.deepEqual(setAdvancedSettingValue({ reasoning: { effort: "max", summary: "auto" } }, mode, "pro"), {
    reasoning: { effort: "max", summary: "auto", mode: "pro" },
  });
  assert.deepEqual(setAdvancedSettingValue({ reasoning: { effort: "max", summary: "auto", mode: "pro" } }, mode, "standard"), {
    reasoning: { effort: "max", summary: "auto" },
  });
});

test("resolveAdvancedSettings exposes OpenRouter Responses reasoning controls", () => {
  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "openrouter_responses",
      options: { temperature: 0.5, reasoning: { effort: "high" } },
      defaultOptions: {},
      policy: allowAdvancedPolicy,
    }).map((item) => [item.kind, item.key, item.value]),
    [
      ["temperature", "temperature", 0.5],
      ["reasoningEffort", "reasoning.effort", "high"],
    ],
  );
});

test("resolveAdvancedSettings maps Anthropic effort with high default", () => {
  const effort = resolveAdvancedSettings({
    protocol: "anthropic_messages",
    options: {},
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "reasoningEffort");
  assert.ok(effort);
  assert.equal(effort.key, "output_config.effort");
  assert.equal(effort.value, "high");

  assert.deepEqual(setAdvancedSettingValue({ output_config: { format: { type: "json" } } }, effort, "max"), {
    output_config: {
      format: { type: "json" },
      effort: "max",
    },
  });
});

test("OpenAI Responses reasoning summary defaults to auto and none prunes only the selected nested value", () => {
  const summary = resolveAdvancedSettings({
    protocol: "openai_responses",
    options: {},
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "reasoningSummary");
  assert.ok(summary);
  assert.equal(summary.value, "auto");
  assert.deepEqual(summary.values, ["none", "auto", "concise", "detailed"]);

  const configured = setAdvancedSettingValue({ reasoning: { effort: "high" } }, summary, "detailed");
  assert.deepEqual(configured, { reasoning: { effort: "high", summary: "detailed" } });
  assert.deepEqual(setAdvancedSettingValue(configured, summary, "none"), { reasoning: { effort: "high" } });
  assert.deepEqual(setAdvancedSettingValue({}, summary, "none"), {});
});

test("Anthropic speed uses standard as an omitted UI default", () => {
  const speed = resolveAdvancedSettings({
    protocol: "anthropic_messages",
    options: {},
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "speed");
  assert.ok(speed);
  assert.equal(speed.key, "speed");
  assert.equal(speed.value, "standard");
  assert.deepEqual(speed.values, ["standard", "fast"]);
  assert.deepEqual(setAdvancedSettingValue({ top_k: 20 }, speed, "fast"), { top_k: 20, speed: "fast" });
  assert.deepEqual(setAdvancedSettingValue({ top_k: 20, speed: "fast" }, speed, "standard"), { top_k: 20 });
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

test("resolveAdvancedSettings exposes fixed GPT Image 2 quality and resolution options for OpenAI Images", () => {
  const settings = resolveAdvancedSettings({
    protocol: "openai_image_generations",
    modelName: "GPT Image 2",
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

  assert.equal(resolution?.customValueKind, undefined);
  assert.deepEqual(setAdvancedSettingValue({}, resolution!, "1600x1024"), {});
  assert.deepEqual(setAdvancedSettingValue({ size: "1024x1024" }, resolution!, "1000x1000"), {
    size: "1024x1024",
  });

  const editResolution = resolveAdvancedSettings({
    protocol: "openai_image_edits",
    modelName: "dall-e-3",
    options: {},
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "imageResolution");
  assert.deepEqual(editResolution?.values, resolution?.values);
  assert.equal(editResolution?.customValueKind, undefined);

  const generationFormat = settings.find((item) => item.kind === "outputFormat");
  assert.equal(generationFormat?.key, "output_format");
  assert.equal(generationFormat?.value, "png");
  assert.deepEqual(generationFormat?.values, ["png", "jpeg", "webp"]);
  assert.equal(
    resolveAdvancedSettings({
      protocol: "openai_image_edits",
      options: {},
      defaultOptions: {},
      policy: allowAdvancedPolicy,
    }).some((item) => item.kind === "outputFormat"),
    false,
  );
});

test("xAI image generation and edit expose aspect ratio, resolution, and response format", () => {
  for (const protocol of ["xai_image", "xai_image_edits"]) {
    const settings = resolveAdvancedSettings({
      protocol,
      options: { aspect_ratio: "16:9", resolution: "2k", response_format: "url" },
      defaultOptions: {},
      policy: allowAdvancedPolicy,
    });
    assert.deepEqual(
      settings.map((item) => [item.kind, item.key, item.value]),
      [
        ["imageAspectRatio", "aspect_ratio", "16:9"],
        ["imageResolution", "resolution", "2k"],
        ["responseFormat", "response_format", "url"],
      ],
    );
    const aspectRatio = settings.find((item) => item.kind === "imageAspectRatio");
    const resolution = settings.find((item) => item.kind === "imageResolution");
    const responseFormat = settings.find((item) => item.kind === "responseFormat");
    assert.ok(aspectRatio);
    assert.ok(resolution);
    assert.ok(responseFormat);
    assert.deepEqual(aspectRatio.values, [
      "auto",
      "1:1",
      "16:9",
      "9:16",
      "4:3",
      "3:4",
      "3:2",
      "2:3",
      "2:1",
      "1:2",
      "19.5:9",
      "9:19.5",
      "20:9",
      "9:20",
    ]);
    assert.deepEqual(resolution.values, ["1k", "2k"]);
    assert.deepEqual(responseFormat.values, ["url", "b64_json"]);
    assert.deepEqual(
      setAdvancedSettingValue({ aspect_ratio: "16:9", resolution: "2k" }, aspectRatio, "auto"),
      { resolution: "2k" },
    );
  }
});

test("Gemini Interactions exposes media settings only for the active task", () => {
  const base = {
    protocol: "gemini_interactions",
    options: {},
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  } as const;
  assert.deepEqual(
    resolveAdvancedSettings(base).map((item) => [item.kind, item.key, item.value]),
    [["thinkingLevel", "generation_config.thinking_level", "high"]],
  );
  assert.deepEqual(
    resolveAdvancedSettings({ ...base, submitTask: "image_generation" }).map((item) => [item.kind, item.key, item.value]),
    [
      ["thinkingLevel", "generation_config.thinking_level", "high"],
      ["imageSize", "response_format.image_size", "1K"],
    ],
  );
  assert.deepEqual(
    resolveAdvancedSettings({ ...base, submitTask: "image_edit" }).map((item) => item.key),
    ["generation_config.thinking_level", "response_format.image_size"],
  );
  const videoSettings = resolveAdvancedSettings({ ...base, submitTask: "video_generation" });
  assert.deepEqual(
    videoSettings.map((item) => [item.kind, item.key, item.value]),
    [
      ["thinkingLevel", "generation_config.thinking_level", "high"],
      ["videoTask", "generation_config.video_config.task", "auto"],
    ],
  );
  const videoTask = videoSettings.find((item) => item.kind === "videoTask");
  assert.ok(videoTask);
  assert.deepEqual(videoTask.values, ["auto", "text_to_video", "image_to_video", "reference_to_video", "edit"]);
  assert.deepEqual(
    setAdvancedSettingValue({ generation_config: { thinking_level: "low", video_config: { task: "edit" } } }, videoTask, "auto"),
    { generation_config: { thinking_level: "low" } },
  );
});

test("legacy built-in policy restores Gemini Interactions advanced settings without changing custom policies", () => {
  const legacy = JSON.parse(DEFAULT_MODEL_OPTION_ALLOWED_PATHS) as Record<string, string[]>;
  delete legacy.gemini_interactions;
  const legacyJSON = JSON.stringify(legacy);
  const normalized = JSON.parse(normalizeModelOptionAllowedPathsJSON(legacyJSON)) as Record<string, string[]>;
  assert.ok(normalized.gemini_interactions.includes("generation_config.thinking_level"));
  assert.ok(normalized.gemini_interactions.includes("generation_config.video_config.task"));

  const policy: ModelOptionPolicy = {
    ...allowAdvancedPolicy,
    allowedPathsJSON: legacyJSON,
  };
  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "gemini_interactions",
      submitTask: "video_generation",
      options: {},
      defaultOptions: {},
      policy,
    }).map((item) => item.key),
    ["generation_config.thinking_level", "generation_config.video_config.task"],
  );

  legacy.openai_chat_completions.push("metadata.tenant");
  const customizedJSON = JSON.stringify(legacy);
  const normalizedCustom = JSON.parse(
    normalizeModelOptionAllowedPathsJSON(customizedJSON),
  ) as Record<string, string[]>;
  assert.equal(normalizedCustom.gemini_interactions, undefined);
  assert.ok(normalizedCustom.openai_chat_completions.includes("metadata.tenant"));

  const protocolCustomized = JSON.parse(DEFAULT_MODEL_OPTION_ALLOWED_PATHS) as Record<string, string[]>;
  delete protocolCustomized.gemini_interactions;
  delete protocolCustomized.openrouter_chat_completions;
  const normalizedProtocolCustom = JSON.parse(
    normalizeModelOptionAllowedPathsJSON(JSON.stringify(protocolCustomized)),
  ) as Record<string, string[]>;
  assert.equal(normalizedProtocolCustom.gemini_interactions, undefined);
});

test("pre-OpenRouter built-in policy restores Gemini Interactions video settings", () => {
  const legacyJSON = JSON.stringify(preOpenRouterModelOptionAllowedPaths);
  const normalized = JSON.parse(
    normalizeModelOptionAllowedPathsJSON(legacyJSON),
  ) as Record<string, string[]>;
  assert.ok(normalized.gemini_interactions.includes("generation_config.thinking_level"));
  assert.ok(normalized.gemini_interactions.includes("generation_config.video_config.task"));

  const policy: ModelOptionPolicy = {
    ...allowAdvancedPolicy,
    allowedPathsJSON: legacyJSON,
  };
  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "gemini_interactions",
      submitTask: "video_generation",
      options: {},
      defaultOptions: {},
      policy,
    }).map((item) => item.key),
    ["generation_config.thinking_level", "generation_config.video_config.task"],
  );

  const custom = JSON.parse(legacyJSON) as Record<string, string[]>;
  custom.openai_chat_completions.push("metadata.tenant");
  const normalizedCustom = JSON.parse(
    normalizeModelOptionAllowedPathsJSON(JSON.stringify(custom)),
  ) as Record<string, string[]>;
  assert.equal(normalizedCustom.gemini_interactions, undefined);
  assert.ok(normalizedCustom.openai_chat_completions.includes("metadata.tenant"));
});

test("resolveAdvancedSettings exposes Gemini image resolution, aspect ratio, and thinking controls", () => {
  const settings = resolveAdvancedSettings({
    protocol: "google_image_generation",
    modelName: "gemini-3.1-flash-image",
    options: {
      generationConfig: {
        imageConfig: { imageSize: "4K", aspectRatio: "16:9" },
        thinkingConfig: { thinkingLevel: "minimal" },
      },
    },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  });
  const resolution = settings.find((item) => item.kind === "imageResolution");
  const aspectRatio = settings.find((item) => item.kind === "imageAspectRatio");
  const thinking = settings.find((item) => item.kind === "reasoningEffort");

  assert.equal(resolution?.key, "generationConfig.imageConfig.imageSize");
  assert.deepEqual(resolution?.values, ["512", "1K", "2K", "4K"]);
  assert.equal(resolution?.value, "4K");
  assert.equal(aspectRatio?.key, "generationConfig.imageConfig.aspectRatio");
  assert.deepEqual(aspectRatio?.values, ["auto", "1:1", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16", "16:9", "21:9", "1:4", "4:1", "1:8", "8:1"]);
  assert.equal(aspectRatio?.value, "16:9");
  assert.equal(thinking?.key, "generationConfig.thinkingConfig.thinkingLevel");
  assert.deepEqual(thinking?.values, ["minimal", "high"]);
  assert.equal(thinking?.value, "minimal");

  assert.deepEqual(
    setAdvancedSettingValue({ generationConfig: { imageConfig: { imageSize: "2K", aspectRatio: "16:9" } } }, aspectRatio!, "auto"),
    { generationConfig: { imageConfig: { imageSize: "2K" } } },
  );
});

test("resolveAdvancedSettings exposes Sora video resolution and seconds options", () => {
  const baseSettings = resolveAdvancedSettings({
    protocol: "openai_video_generations",
    modelName: "sora-2",
    options: { size: "720x1280", seconds: "8" },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  });
  const baseResolution = baseSettings.find((item) => item.kind === "videoResolution");
  const baseSeconds = baseSettings.find((item) => item.kind === "videoSeconds");

  assert.equal(baseResolution?.key, "size");
  assert.equal(baseResolution?.value, "720x1280");
  assert.deepEqual(baseResolution?.values, ["720x1280", "1280x720"]);
  assert.equal(baseSeconds?.key, "seconds");
  assert.equal(baseSeconds?.value, "8");
  assert.deepEqual(baseSeconds?.values, ["4", "8", "12"]);
  assert.deepEqual(setAdvancedSettingValue({}, baseSeconds!, "16"), {});

  const proResolution = resolveAdvancedSettings({
    protocol: "openai_video_generations",
    modelName: "sora-2-pro",
    options: { size: "1920x1080", seconds: "12" },
    defaultOptions: {},
    policy: allowAdvancedPolicy,
  }).find((item) => item.kind === "videoResolution");

  assert.deepEqual(proResolution?.values, [
    "720x1280",
    "1280x720",
    "1024x1792",
    "1792x1024",
    "1080x1920",
    "1920x1080",
  ]);
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

test("resolveAdvancedSettings upgrades legacy built-in policy paths without changing custom rules", () => {
  const legacyDefaultPolicy: ModelOptionPolicy = {
    ...allowAdvancedPolicy,
    allowedPathsJSON: JSON.stringify({
      default: ["temperature"],
      openai_chat_completions: [
        "service_tier",
        "presence_penalty",
        "frequency_penalty",
        "reasoning_effort",
        "verbosity",
        "thinking.type",
        "stream_options.include_usage",
      ],
      openai_responses: ["service_tier", "reasoning.effort", "reasoning.summary", "text.verbosity"],
      anthropic_messages: ["speed", "top_k", "thinking.type"],
      gemini_generate_content: [
        "generationConfig.temperature",
        "generationConfig.topP",
        "generationConfig.maxOutputTokens",
        "generationConfig.responseMimeType",
      ],
      google_image_generation: [
        "generationConfig.responseModalities",
        "generationConfig.imageConfig.aspectRatio",
        "generationConfig.imageConfig.imageSize",
      ],
    }),
  };

  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "openai_chat_completions",
      options: {},
      defaultOptions: {},
      policy: legacyDefaultPolicy,
    }).map((item) => item.key),
    ["temperature", "reasoning_effort", "reasoning_summary", "verbosity"],
  );

  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "openai_responses",
      options: {},
      defaultOptions: {},
      policy: legacyDefaultPolicy,
    }).map((item) => item.key),
    ["temperature", "reasoning.mode", "reasoning.effort", "reasoning.summary", "text.verbosity"],
  );
  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "anthropic_messages",
      options: {},
      defaultOptions: {},
      policy: legacyDefaultPolicy,
    }).map((item) => item.key),
    ["temperature", "output_config.effort", "speed"],
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
  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "google_image_generation",
      options: {},
      defaultOptions: {},
      policy: legacyDefaultPolicy,
    }).map((item) => item.key),
    [
      "generationConfig.imageConfig.imageSize",
      "generationConfig.imageConfig.aspectRatio",
      "generationConfig.thinkingConfig.thinkingLevel",
    ],
  );

  const customizedOpenAIChatPolicy: ModelOptionPolicy = {
    ...allowAdvancedPolicy,
    allowedPathsJSON: JSON.stringify({
      default: ["temperature"],
      openai_chat_completions: [
        "service_tier",
        "presence_penalty",
        "frequency_penalty",
        "reasoning_effort",
        "verbosity",
        "thinking.type",
        "stream_options.include_usage",
        "metadata.tenant",
      ],
    }),
  };
  assert.deepEqual(
    resolveAdvancedSettings({
      protocol: "openai_chat_completions",
      options: {},
      defaultOptions: {},
      policy: customizedOpenAIChatPolicy,
    }).map((item) => item.key),
    ["temperature", "reasoning_effort", "verbosity"],
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
    reasoning: { effort: "low" },
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
