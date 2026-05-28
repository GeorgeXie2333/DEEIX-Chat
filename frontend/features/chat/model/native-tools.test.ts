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
  assert.deepEqual(
    resolveNativeToolGroup("openai_chat_completions", false)?.options.map((tool) => tool.type),
    ["web_search", "web_search_preview"],
  );
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
