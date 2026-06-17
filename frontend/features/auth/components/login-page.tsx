"use client";

import { ArrowLeftRight, ShieldCheck, Sparkles, type LucideIcon } from "lucide-react";
import { motion, useReducedMotion, type MotionProps } from "motion/react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { PASSWORD_MIN_LENGTH } from "@/shared/auth/account-policy";
import { useLoginPage } from "@/features/auth/hooks/use-auth-login-page";
import type { LoginMode } from "@/features/auth/model/login-page";
import { AppLogo } from "@/shared/components/app-logo";
import { IdentityProviderIcon } from "@/shared/components/identity-provider-icon";
import { TurnstileWidget } from "@/features/auth/components/turnstile-widget";
import { cn } from "@/lib/utils";

type LoginPageProps = {
  nextPath: string;
};

type LoginMessages = ReturnType<typeof useTranslations>;

const LANDING_CAPABILITIES: ReadonlyArray<{ key: string; Icon: LucideIcon }> = [
  { key: "capabilityRouting", Icon: Sparkles },
  { key: "capabilityFiles", Icon: ArrowLeftRight },
  { key: "capabilityUsage", Icon: ShieldCheck },
];
const MODEL_NAMES = ["GPT", "Gemini", "Claude", "Grok"] as const;

// Ambient brand-color wash behind the cards: two large radial glows driven by the
// theme `--primary` token, so it adapts across all 8 themes × dark mode for free.
const AMBIENT_BACKGROUND =
  "radial-gradient(80rem 50rem at 85% -10%, color-mix(in oklch, var(--primary) 6%, transparent), transparent 60%)," +
  "radial-gradient(60rem 40rem at -10% 110%, color-mix(in oklch, var(--primary) 4%, transparent), transparent 60%)";

const ENTRANCE_EASE = [0.16, 1, 0.3, 1] as const;

function entranceProps(reduce: boolean, delay: number): MotionProps {
  if (reduce) {
    return {};
  }
  return {
    initial: { opacity: 0, y: 12 },
    animate: { opacity: 1, y: 0 },
    transition: { duration: 0.4, ease: ENTRANCE_EASE, delay },
  };
}

function LoginBrandMark({ className = "mx-auto h-14" }: { className?: string }) {
  return <AppLogo height={72} priority className={className} />;
}

function LoginLandingCopy({ t, motionProps }: { t: LoginMessages; motionProps: MotionProps }) {
  return (
    <motion.div className="min-w-0" {...motionProps}>
      <LoginBrandMark className="h-12 justify-start text-[1.375rem] text-foreground md:h-14 md:text-[1.625rem]" />
      <div className="mt-6 max-w-[680px] md:mt-12">
        <h1 className="max-w-[15ch] text-3xl font-semibold leading-[1.18] tracking-normal text-foreground md:text-[calc(3.25rem*var(--ui-font-scale))] md:leading-[1.15]">
          {t("landing.headline")}
        </h1>
        <p className="mt-4 max-w-[590px] text-sm leading-6 text-muted-foreground md:mt-5 md:text-lg md:leading-8">
          {t("landing.description")}
        </p>
      </div>
      <ul className="mt-5 hidden gap-2.5 sm:mt-8 sm:grid sm:grid-cols-3 sm:gap-3 lg:max-w-[680px]">
        {LANDING_CAPABILITIES.map(({ key, Icon }) => (
          <li
            key={key}
            className="flex min-w-0 items-center gap-3 rounded-lg border border-border/60 bg-card/80 px-3 py-2.5 text-sm font-medium text-foreground shadow-none sm:py-3"
          >
            <span className="flex size-7 shrink-0 items-center justify-center rounded-md text-muted-foreground">
              <Icon className="size-5" strokeWidth={1.7} aria-hidden />
            </span>
            <span className="min-w-0 leading-5">{t(`landing.${key}`)}</span>
          </li>
        ))}
      </ul>
    </motion.div>
  );
}

function LoginProductPreview({ t, motionProps }: { t: LoginMessages; motionProps: MotionProps }) {
  const previewItems = [
    ["01", t("landing.previewRouting"), t("landing.previewRoutingDescription")],
    ["02", t("landing.previewContext"), t("landing.previewContextDescription")],
    ["03", t("landing.previewGovernance"), t("landing.previewGovernanceDescription")],
  ] as const;

  return (
    <motion.section className="min-w-0" aria-label={t("landing.proofTitle")} {...motionProps}>
      <div className="mb-4 flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h2 className="text-base font-semibold tracking-normal text-foreground">{t("landing.proofTitle")}</h2>
          <p className="mt-1 max-w-[34rem] text-sm leading-6 text-muted-foreground">{t("landing.proofDescription")}</p>
        </div>
      </div>
      <div className="rounded-2xl border border-border/60 bg-card/80 p-4 shadow-sm shadow-foreground/5 backdrop-blur-sm sm:p-5">
        <div className="rounded-xl bg-muted/40 p-3">
          <div className="flex items-center justify-between gap-3 border-b border-border/50 pb-3">
            <div className="min-w-0">
              <div className="text-sm font-semibold leading-5 text-foreground">{t("landing.previewPanelTitle")}</div>
              <div className="mt-0.5 text-xs leading-5 text-muted-foreground">{t("landing.previewPanelDescription")}</div>
            </div>
            <div className="hidden shrink-0 rounded-md bg-primary/10 px-2 py-1 text-xs font-semibold text-primary sm:block">
              {t("landing.previewPanelStatus")}
            </div>
          </div>
          <div className="mt-3 grid grid-cols-2 gap-2 sm:grid-cols-4">
            {MODEL_NAMES.map((name, index) => (
              <div
                key={name}
                className={cn(
                  "rounded-lg border px-3 py-2 text-center text-sm font-semibold transition-colors",
                  index === 0
                    ? "border-primary/40 bg-primary/10 text-primary"
                    : "border-border/50 bg-muted/60 text-foreground",
                )}
              >
                {name}
              </div>
            ))}
          </div>
          <div className="mt-3 divide-y divide-border/40">
            {previewItems.map(([number, title, description]) => (
              <div
                key={number}
                className="grid grid-cols-[2.25rem_minmax(0,1fr)] gap-3 py-3 first:pt-0 last:pb-0"
              >
                <span className="flex size-8 items-center justify-center rounded-md bg-background text-xs font-semibold text-muted-foreground">
                  {number}
                </span>
                <span className="min-w-0">
                  <span className="block text-sm font-semibold leading-5 text-foreground">{title}</span>
                  <span className="mt-0.5 block text-xs leading-5 text-muted-foreground">{description}</span>
                </span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </motion.section>
  );
}

function LoginAuthPanelHeader({ t, mode }: { t: LoginMessages; mode: LoginMode }) {
  const title = mode === "register"
    ? t("landing.registerTitle")
    : mode === "reset-password"
      ? t("resetPassword")
      : t("landing.authTitle");
  const description = mode === "register"
    ? t("landing.registerDescription")
    : t("landing.authDescription");

  return (
    <div className="space-y-2 text-left">
      <h2 className="text-2xl font-semibold leading-8 tracking-normal text-foreground">
        {title}
      </h2>
      <p className="text-sm leading-6 text-muted-foreground">
        {description}
      </p>
    </div>
  );
}

export function LoginPage({ nextPath }: LoginPageProps) {
  const t = useTranslations("login");
  const reduceMotion = useReducedMotion() ?? false;
  const entrance = (delay: number) => entranceProps(reduceMotion, delay);
  const loginPage = useLoginPage({ nextPath });
  const {
    cancelTwoFactorChallenge,
    canShowRegisterSwitch,
    codeSent,
    configReady,
    emailRegistrationEnabled,
    emailVerificationEnabled,
    handleProviderLogin,
    loginProviders,
    mode,
    onLoginSubmit,
    onRegisterSubmit,
    options,
    password,
    passwordLoginEnabled,
    passwordResetEnabled,
    registerCode,
    registerCodeCooldownSeconds,
    registerDebugCode,
    registerEmail,
    registerPassword,
    registerTurnstileRequired,
    registerTurnstileResetSignal,
    registerTurnstileSiteKey,
    registerTurnstileToken,
    requestRegisterCode,
    requestPasswordResetCode,
    requestTwoFactorEmailCode,
    resetCode,
    resetCodeCooldownSeconds,
    resetCodeSent,
    resetEmail,
    resetPassword,
    sendingCode,
    setPassword,
    setRegisterCode,
    setRegisterPassword,
    setRegisterTurnstileToken,
    setResetCode,
    setResetPassword,
    setTwoFactorCode,
    switchTwoFactorVerificationMethod,
    setUsername,
    submitting,
    toggleLoginMode,
    twoFactorChallengeToken,
    twoFactorCode,
    twoFactorEmailCodeCooldownSeconds,
    twoFactorEmailDebugCode,
    twoFactorVerificationMethod,
    twoFactorVerificationMethods,
    updateResetEmail,
    updateRegisterEmail,
    username,
    onPasswordResetSubmit,
  } = loginPage;

  const accountLabel = options.emailEnabled && options.usernameEnabled
    ? t("account")
    : options.emailEnabled
      ? t("email")
      : t("username");
  const accountPlaceholder = options.emailEnabled && options.usernameEnabled
    ? t("emailOrUsername")
    : options.emailEnabled
      ? t("email")
      : t("username");
  const alternativeTwoFactorMethod = twoFactorVerificationMethods.find((method) => method !== twoFactorVerificationMethod && method !== "none");
  const twoFactorUsesEmail = twoFactorVerificationMethod === "email";
  const modeKey = mode === "reset-password" ? "reset-password" : mode === "register" ? "register" : twoFactorChallengeToken ? "twofactor" : "credentials";

  const modeContent = (
    <>
      {mode === "login" && twoFactorChallengeToken ? (
        <>
          <form className="mt-6 space-y-4" onSubmit={onLoginSubmit}>
            <div className="space-y-2">
              <label className="text-sm font-medium leading-none text-foreground" htmlFor="otp">
                {twoFactorUsesEmail ? t("verificationCode") : t("twoFactorCode")}
              </label>
              <div className="flex gap-2">
                <Input
                  id="otp"
                  name="otp"
                  type="text"
                  inputMode={twoFactorUsesEmail ? "numeric" : "text"}
                  autoComplete="one-time-code"
                  pattern={twoFactorUsesEmail ? "[0-9]*" : undefined}
                  className="h-10 min-w-0 border-input/70 bg-background/70 text-sm"
                  placeholder={twoFactorUsesEmail ? t("verificationCodePlaceholder") : t("twoFactorPlaceholder")}
                  value={twoFactorCode}
                  onChange={(event) => setTwoFactorCode(event.target.value)}
                  required
                />
                {twoFactorUsesEmail ? (
                  <Button
                    type="button"
                    variant="secondary"
                    className="h-10 min-w-[4.5rem] shrink-0 rounded-lg border border-border/50 bg-muted/60 px-3 text-sm font-semibold text-foreground shadow-none hover:bg-muted"
                    disabled={sendingCode || twoFactorEmailCodeCooldownSeconds > 0}
                    onClick={() => {
                      void requestTwoFactorEmailCode();
                    }}
                  >
                    {sendingCode ? <SpinnerLabel>{t("sending")}</SpinnerLabel> : twoFactorEmailCodeCooldownSeconds > 0 ? t("resendIn", { seconds: twoFactorEmailCodeCooldownSeconds }) : t("send")}
                  </Button>
                ) : null}
              </div>
              {twoFactorEmailDebugCode ? <p className="text-xs font-medium text-muted-foreground">{t("debugCode", { code: twoFactorEmailDebugCode })}</p> : null}
            </div>
            <Button
              className="mt-1 h-10 w-full rounded-lg bg-primary text-sm font-semibold text-primary-foreground shadow-none hover:bg-primary/90"
              type="submit"
              disabled={submitting}
            >
              {submitting ? <SpinnerLabel>{t("signingIn")}</SpinnerLabel> : t("verifyAndSignIn")}
            </Button>
          </form>
          {alternativeTwoFactorMethod ? (
            <Button
              type="button"
              variant="ghost"
              className="mt-2 h-9 w-full text-xs text-muted-foreground shadow-none"
              onClick={() => switchTwoFactorVerificationMethod(alternativeTwoFactorMethod)}
            >
              {alternativeTwoFactorMethod === "email" ? t("useEmailVerification") : t("useTwoFactorVerification")}
            </Button>
          ) : null}
          <Button
            type="button"
            variant="ghost"
            className="mt-2 h-9 w-full text-xs text-muted-foreground shadow-none"
            onClick={cancelTwoFactorChallenge}
          >
            {passwordLoginEnabled ? t("backToPasswordLogin") : t("backToLoginMethods")}
          </Button>
        </>
      ) : null}

      {mode === "login" && !twoFactorChallengeToken && passwordLoginEnabled ? (
        <form className="mt-6 space-y-4" onSubmit={onLoginSubmit}>
          <div className="space-y-2">
            <label className="text-sm font-medium leading-none text-foreground" htmlFor="username">
              {accountLabel}
            </label>
            <Input
              id="username"
              name="username"
              autoComplete="username"
              className="h-10 border-input/70 bg-background/70 text-sm"
              placeholder={accountPlaceholder}
              value={username}
              onChange={(event) => setUsername(event.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3">
              <label className="text-sm font-medium leading-none text-foreground" htmlFor="password">
                {t("password")}
              </label>
              {passwordResetEnabled ? (
                <button
                  type="button"
                  className="text-xs font-medium text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline focus-visible:text-foreground focus-visible:underline focus-visible:outline-none"
                  onClick={() => {
                    updateResetEmail(username.includes("@") ? username : "");
                    loginPage.setMode("reset-password");
                  }}
                >
                  {t("forgotPassword")}
                </button>
              ) : null}
            </div>
            <Input
              id="password"
              name="password"
              type="password"
              autoComplete="current-password"
              className="h-10 border-input/70 bg-background/70 text-sm"
              placeholder={t("password")}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </div>
          <Button
            className="mt-1 h-10 w-full rounded-lg bg-primary text-sm font-semibold text-primary-foreground shadow-none hover:bg-primary/90"
            type="submit"
            disabled={submitting}
          >
            {submitting ? <SpinnerLabel>{t("signingIn")}</SpinnerLabel> : t("signIn")}
          </Button>
        </form>
      ) : null}

      {mode === "reset-password" && passwordResetEnabled ? (
        <>
          <form className="mt-6 space-y-4" onSubmit={onPasswordResetSubmit}>
            <div className="space-y-2">
              <label className="text-sm font-medium leading-none text-foreground" htmlFor="reset-email">
                {t("email")}
              </label>
              <Input
                id="reset-email"
                type="email"
                autoComplete="email"
                className="h-10 border-input/70 bg-background/70 text-sm"
                placeholder={t("email")}
                value={resetEmail}
                onChange={(event) => updateResetEmail(event.target.value)}
                required
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium leading-none text-foreground" htmlFor="reset-password">
                {t("newPassword")}
              </label>
              <Input
                id="reset-password"
                type="password"
                autoComplete="new-password"
                className="h-10 border-input/70 bg-background/70 text-sm"
                placeholder={t("newPasswordPlaceholder")}
                value={resetPassword}
                onChange={(event) => setResetPassword(event.target.value)}
                minLength={PASSWORD_MIN_LENGTH}
                required
              />
            </div>
            <div className="space-y-2">
              <label className="text-sm font-medium leading-none text-foreground" htmlFor="reset-code">
                {t("verificationCode")}
              </label>
              <div className="flex gap-2">
                <Input
                  id="reset-code"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  className="h-10 border-input/70 bg-background/70 text-sm"
                  placeholder={t("verificationCodePlaceholder")}
                  value={resetCode}
                  onChange={(event) => setResetCode(event.target.value)}
                  required
                />
                <Button
                  type="button"
                  variant="secondary"
                  className="h-10 min-w-[4.5rem] shrink-0 rounded-lg border border-border/50 bg-muted/60 px-3 text-sm font-semibold text-foreground shadow-none hover:bg-muted"
                  disabled={sendingCode || resetCodeCooldownSeconds > 0 || !resetEmail.trim()}
                  onClick={() => {
                    void requestPasswordResetCode();
                  }}
                >
                  {sendingCode ? <SpinnerLabel>{t("sending")}</SpinnerLabel> : resetCodeCooldownSeconds > 0 ? t("resendIn", { seconds: resetCodeCooldownSeconds }) : resetCodeSent ? t("resend") : t("send")}
                </Button>
              </div>
            </div>
            <Button
              className="mt-1 h-10 w-full rounded-lg bg-primary text-sm font-semibold text-primary-foreground shadow-none hover:bg-primary/90"
              type="submit"
              disabled={submitting || resetCode.length !== 6}
            >
              {submitting ? <SpinnerLabel>{t("resettingPassword")}</SpinnerLabel> : t("resetPassword")}
            </Button>
          </form>
          <div className="mt-6 text-center text-sm font-normal leading-5 text-muted-foreground">
            {t("rememberPassword")}{" "}
            <button
              type="button"
              className="font-semibold text-foreground underline-offset-4 hover:underline focus-visible:underline focus-visible:outline-none"
              onClick={() => loginPage.setMode("login")}
            >
              {t("back")}
            </button>
          </div>
        </>
      ) : null}

      {mode === "register" && emailRegistrationEnabled ? (
        <form className="mt-6 space-y-4" onSubmit={onRegisterSubmit}>
          <div className="space-y-2">
            <label className="text-sm font-medium leading-none text-foreground" htmlFor="register-email">
              {t("email")}
            </label>
            <Input
              id="register-email"
              type="email"
              autoComplete="email"
              className="h-10 border-input/70 bg-background/70 text-sm"
              placeholder={t("email")}
              value={registerEmail}
              onChange={(event) => updateRegisterEmail(event.target.value)}
              required
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-medium leading-none text-foreground" htmlFor="register-password">
              {t("password")}
            </label>
            <Input
              id="register-password"
              type="password"
              autoComplete="new-password"
              className="h-10 border-input/70 bg-background/70 text-sm"
              placeholder={t("newPasswordPlaceholder")}
              value={registerPassword}
              onChange={(event) => setRegisterPassword(event.target.value)}
              minLength={PASSWORD_MIN_LENGTH}
              required
            />
          </div>
          {registerTurnstileRequired ? (
            <TurnstileWidget
              siteKey={registerTurnstileSiteKey}
              resetSignal={registerTurnstileResetSignal}
              onTokenChange={setRegisterTurnstileToken}
            />
          ) : null}
          {emailVerificationEnabled ? (
            <div className="space-y-2">
              <label className="text-sm font-medium leading-none text-foreground" htmlFor="register-code">
                {t("verificationCode")}
              </label>
              <div className="flex gap-2">
                <Input
                  id="register-code"
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  className="h-10 border-input/70 bg-background/70 text-sm"
                  placeholder={t("verificationCodePlaceholder")}
                  value={registerCode}
                  onChange={(event) => setRegisterCode(event.target.value)}
                  required
                />
                <Button
                  type="button"
                  variant="secondary"
                  className="h-10 min-w-[4.5rem] shrink-0 rounded-lg border border-border/50 bg-muted/60 px-3 text-sm font-semibold text-foreground shadow-none hover:bg-muted"
                  disabled={sendingCode || registerCodeCooldownSeconds > 0 || !registerEmail.trim() || (registerTurnstileRequired && !registerTurnstileToken)}
                  onClick={() => {
                    void requestRegisterCode();
                  }}
                >
                  {sendingCode ? <SpinnerLabel>{t("sending")}</SpinnerLabel> : registerCodeCooldownSeconds > 0 ? t("resendIn", { seconds: registerCodeCooldownSeconds }) : codeSent ? t("resend") : t("send")}
                </Button>
              </div>
            </div>
          ) : null}
          {registerDebugCode ? <p className="text-xs font-medium text-muted-foreground">{t("debugCode", { code: registerDebugCode })}</p> : null}
          <Button
            className="mt-1 h-10 w-full rounded-lg bg-primary text-sm font-semibold text-primary-foreground shadow-none hover:bg-primary/90"
            type="submit"
            disabled={submitting || (emailVerificationEnabled && registerCode.length !== 6) || (!emailVerificationEnabled && registerTurnstileRequired && !registerTurnstileToken)}
          >
            {submitting ? <SpinnerLabel>{t("registering")}</SpinnerLabel> : t("register")}
          </Button>
        </form>
      ) : null}

      {mode === "login" && !twoFactorChallengeToken && loginProviders.length > 0 ? (
        <>
          {passwordLoginEnabled ? (
            <div className="my-6 flex items-center gap-3">
              <span className="h-px flex-1 bg-border/60" aria-hidden />
              <span className="text-xs font-medium text-muted-foreground">{t("orContinueWith")}</span>
              <span className="h-px flex-1 bg-border/60" aria-hidden />
            </div>
          ) : null}
          <div className={cn("space-y-2.5", passwordLoginEnabled ? undefined : "mt-7")}>
            {loginProviders.map((provider) => (
              <Button
                key={provider.publicID}
                type="button"
                variant="secondary"
                className="h-10 w-full rounded-lg border border-border/50 bg-muted/60 text-sm font-semibold text-foreground shadow-none hover:bg-muted"
                onClick={() => {
                  void handleProviderLogin(provider.slug);
                }}
              >
                <span className="inline-flex min-w-0 items-center justify-center gap-2">
                  <IdentityProviderIcon
                    name={provider.name}
                    slug={provider.slug}
                    logoURL={provider.logoURL}
                    className="size-5"
                    iconClassName="size-5"
                    fallbackClassName="text-sm font-semibold uppercase text-foreground"
                  />
                  <span className="truncate">{t("providerLogin", { provider: provider.name })}</span>
                </span>
              </Button>
            ))}
          </div>
        </>
      ) : null}

      {canShowRegisterSwitch && mode !== "reset-password" ? (
        <div className="mt-6 text-center text-sm font-normal leading-5 text-muted-foreground">
          {mode === "register" ? t("alreadyHaveAccount") : t("noAccount")}{" "}
          <button
            type="button"
            className="font-semibold text-foreground underline-offset-4 hover:underline"
            onClick={toggleLoginMode}
          >
            {mode === "register" ? t("signIn") : t("register")}
          </button>
        </div>
      ) : null}
    </>
  );

  return (
    <main
      className="h-svh overflow-x-hidden overflow-y-auto bg-background text-foreground"
      style={{ backgroundImage: AMBIENT_BACKGROUND }}
      aria-busy={!configReady}
    >
      <section className="mx-auto grid min-h-full w-full max-w-[1180px] grid-cols-1 gap-5 px-4 py-5 sm:gap-7 sm:px-6 sm:py-6 md:gap-9 md:py-10 lg:grid-cols-[minmax(0,1fr)_minmax(360px,420px)] lg:grid-rows-[auto_auto] lg:items-center lg:gap-x-14 lg:gap-y-8 lg:px-8 xl:px-0">
        <LoginLandingCopy t={t} motionProps={entrance(0)} />

        <motion.div
          className="w-full max-w-[420px] justify-self-center rounded-2xl border border-border/60 bg-card/80 p-5 shadow-xl shadow-foreground/5 backdrop-blur-sm sm:p-8 lg:col-start-2 lg:row-span-2 lg:row-start-1 lg:justify-self-end"
          {...entrance(0.08)}
        >
          <LoginAuthPanelHeader t={t} mode={mode} />

          <div
            aria-hidden={!configReady}
            className={cn(
              "grid transition-[grid-template-rows,opacity] duration-200 ease-out motion-reduce:transition-none",
              configReady ? "grid-rows-[1fr] opacity-100" : "pointer-events-none grid-rows-[0fr] opacity-0",
            )}
          >
            {configReady ? (
              reduceMotion ? (
                <div key={modeKey} className="min-h-0 overflow-hidden">
                  {modeContent}
                </div>
              ) : (
                <motion.div
                  key={modeKey}
                  className="min-h-0 overflow-hidden"
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.2, ease: ENTRANCE_EASE }}
                >
                  {modeContent}
                </motion.div>
              )
            ) : null}
          </div>
        </motion.div>

        <LoginProductPreview t={t} motionProps={entrance(0.16)} />
      </section>
    </main>
  );
}
