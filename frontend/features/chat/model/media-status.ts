export function resolveMediaStatusProgress(raw: unknown): number | undefined {
  if (raw === null || raw === undefined) {
    return undefined;
  }
  const value = typeof raw === "number" ? raw : typeof raw === "string" && raw.trim() ? Number(raw.trim()) : Number.NaN;
  if (!Number.isFinite(value)) {
    return undefined;
  }
  return Math.max(0, Math.min(100, Math.round(value)));
}
