export const PROMPT_PRESET_LIMITS = {
  name: 16,
  description: 64,
  content: 16384,
} as const;

export function normalizePromptPresetName(value: string): string {
  return value
    .trim()
    .replace(/^\/+/, "")
    .trim();
}
