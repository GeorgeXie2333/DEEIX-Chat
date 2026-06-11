export const APP_LOGO_MASK_SRC = "/logo.svg";
export const APP_LOGO_ASPECT_RATIO = 219.07144 / 43.166336;

export function appLogoMaskImage(): `url("${string}")` {
  return `url("${APP_LOGO_MASK_SRC}")`;
}

export function appLogoVisualHeight(height: number): number {
  return Math.max(13, Math.min(32, Math.round(height * 0.42)));
}

export function appLogoVisualWidth(height: number): number {
  return Math.round(appLogoVisualHeight(height) * APP_LOGO_ASPECT_RATIO);
}
