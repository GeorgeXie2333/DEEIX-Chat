import assert from "node:assert/strict";
import test from "node:test";

import {
  countProviderTools,
  hasProviderTool,
  providerToolObjectsFromOptions,
  resolveNativeToolGroup,
  setProviderToolEnabled,
  shouldShowMCPToolsMenu,
  type NativeToolOption,
} from "./native-tools.ts";

test("resolveNativeToolGroup maps supported protocols and hides tools in media mode", () => {
  assert.equal(resolveNativeToolGroup("unknown_protocol", false), null);
  assert.equal(resolveNativeToolGroup("openai_responses", true), null);

  assert.deepEqual(
    resolveNativeToolGroup("xai_responses", false)?.options.map((tool) => tool.type),
    ["web_search", "x_search", "code_interpreter"],
  );
  assert.equal(resolveNativeToolGroup("openai_chat_completions", false), null);
  assert.deepEqual(
    resolveNativeToolGroup("openai_responses", false)?.options.map((tool) => tool.type),
    ["web_search", "shell", "image_generation", "code_interpreter"],
  );
  assert.deepEqual(
    resolveNativeToolGroup("anthropic_messages", false)?.options.map((tool) => tool.type),
    [
      "web_search_20260209",
      "web_fetch_20260209",
      "code_execution_20260120",
      "advisor_20260301",
      "tool_search_tool_regex_20251119",
      "tool_search_tool_bm25_20251119",
    ],
  );
  assert.equal(resolveNativeToolGroup("gemini_generate_content", false, "gemini-2.5-pro"), null);
  assert.deepEqual(
    resolveNativeToolGroup("gemini_generate_content", false, "gemini-3-pro-preview")?.options.map((tool) => tool.type),
    ["google_search", "code_execution"],
  );
});

test("resolveNativeToolGroup marks x_search with the x.com logo icon", () => {
  const xSearch = resolveNativeToolGroup("xai_responses", false)?.options.find((tool) => tool.type === "x_search");

  assert.equal(xSearch?.iconKind, "x-logo");
});

test("provider tool helpers ignore malformed tools and count selected native tool types once", () => {
  const openAIGroup = resolveNativeToolGroup("openai_responses", false);
  const options = {
    tools: [
      { type: "web_search" },
      null,
      "bad",
      { type: "web_search" },
      { type: "shell" },
      { type: "unrelated" },
      ["also-bad"],
    ],
  };

  assert.equal(providerToolObjectsFromOptions({ tools: "bad" }).length, 0);
  assert.deepEqual(providerToolObjectsFromOptions(options), [
    { type: "web_search" },
    { type: "web_search" },
    { type: "shell" },
    { type: "unrelated" },
  ]);
  assert.equal(hasProviderTool(options, "web_search"), true);
  assert.equal(hasProviderTool(options, "image_generation"), false);
  assert.equal(countProviderTools(options, openAIGroup), 2);
  assert.equal(countProviderTools(options, null), 0);
});

test("setProviderToolEnabled adds protocol payloads, preserves unrelated options, and avoids duplicates", () => {
  const shell = resolveNativeToolGroup("openai_responses", false)?.options.find((tool) => tool.type === "shell");
  assert.ok(shell);

  const current = {
    temperature: 0.2,
    tools: [{ type: "unrelated" }],
  };
  const enabled = setProviderToolEnabled(current, shell, true);

  assert.notEqual(enabled, current);
  assert.deepEqual(enabled, {
    temperature: 0.2,
    tools: [
      { type: "unrelated" },
      { type: "shell", environment: { type: "container_auto" } },
    ],
  });

  const enabledAgain = setProviderToolEnabled(enabled, shell, true);
  assert.deepEqual(enabledAgain.tools, enabled.tools);
});

test("Gemini native tools use key-based payloads without unofficial type fields", () => {
  const geminiGroup = resolveNativeToolGroup("gemini_generate_content", false, "gemini-3-pro-preview");
  const googleSearch = geminiGroup?.options.find((tool) => tool.type === "google_search");
  const codeExecution = geminiGroup?.options.find((tool) => tool.type === "code_execution");
  assert.ok(geminiGroup);
  assert.ok(googleSearch);
  assert.ok(codeExecution);

  const enabled = setProviderToolEnabled({ tools: [{ type: "unrelated" }] }, googleSearch, true);
  assert.deepEqual(enabled, {
    tools: [{ type: "unrelated" }, { google_search: {} }],
  });
  assert.equal(hasProviderTool(enabled, "google_search"), true);
  assert.equal(countProviderTools({ tools: [{ google_search: {} }, { type: "google_search" }, { code_execution: {} }] }, geminiGroup), 2);

  const enabledAgain = setProviderToolEnabled(enabled, googleSearch, true);
  assert.deepEqual(enabledAgain.tools, enabled.tools);

  const removed = setProviderToolEnabled({ tools: [{ google_search: {} }, { code_execution: {} }] }, googleSearch, false);
  assert.deepEqual(removed, { tools: [{ code_execution: {} }] });
});

test("setProviderToolEnabled removes only the selected tool and deletes tools when empty", () => {
  const webSearch = resolveNativeToolGroup("openai_responses", false)?.options.find((tool) => tool.type === "web_search");
  const shell = resolveNativeToolGroup("openai_responses", false)?.options.find((tool) => tool.type === "shell");
  assert.ok(webSearch);
  assert.ok(shell);

  assert.deepEqual(
    setProviderToolEnabled({ max_tokens: 100, tools: [{ type: "web_search" }, { type: "shell" }] }, webSearch, false),
    { max_tokens: 100, tools: [{ type: "shell" }] },
  );
  assert.deepEqual(setProviderToolEnabled({ max_tokens: 100, tools: [{ type: "shell" }] }, shell, false), {
    max_tokens: 100,
  });
});

test("setProviderToolEnabled ignores unknown native tool definitions defensively", () => {
  const unknownTool: NativeToolOption = {
    type: "unknown_native_tool",
    labelKey: "unknown",
    descriptionKey: "unknown",
  };
  const current = { tools: [{ type: "web_search" }] };

  assert.equal(setProviderToolEnabled(current, unknownTool, true), current);
  assert.equal(setProviderToolEnabled(current, unknownTool, false), current);
});

test("shouldShowMCPToolsMenu only shows configured MCP tools outside media mode", () => {
  assert.equal(shouldShowMCPToolsMenu(0, false), false);
  assert.equal(shouldShowMCPToolsMenu(-1, false), false);
  assert.equal(shouldShowMCPToolsMenu(1, true), false);
  assert.equal(shouldShowMCPToolsMenu(2, false), true);
});
