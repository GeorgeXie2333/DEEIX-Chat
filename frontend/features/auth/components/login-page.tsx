"use client";

import { motion, useReducedMotion } from "motion/react";
import { useTranslations } from "next-intl";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { SpinnerLabel } from "@/components/ui/spinner";
import { TurnstileWidget } from "@/features/auth/components/turnstile-widget";
import { useLoginPage } from "@/features/auth/hooks/use-auth-login-page";
import type { LoginMode } from "@/features/auth/model/login-page";
import { cn } from "@/lib/utils";
import { PASSWORD_MIN_LENGTH } from "@/shared/auth/account-policy";
import { IdentityProviderIcon } from "@/shared/components/identity-provider-icon";
import { PublicBrandSurface } from "@/shared/components/public-brand-surface";
import { PublicPageHeader } from "@/shared/components/public-page-header";

type LoginPageProps = {
  nextPath: string;
};

type LoginMessages = ReturnType<typeof useTranslations>;

const AMBIENT_BACKGROUND =
  "radial-gradient(52rem 38rem at 50% -15%, color-mix(in oklch, var(--primary) 8%, transparent), transparent 68%)," +
  "radial-gradient(42rem 34rem at 0% 100%, color-mix(in oklch, var(--foreground) 3%, transparent), transparent 72%)";

const ENTRANCE_EASE = [0.16, 1, 0.3, 1] as const;

function LoginAuthPanelHeader({
  t,
  mode,
  twoFactorActive,
}: {
  t: LoginMessages;
  mode: LoginMode;
  twoFactorActive: boolean;
}) {
  const title = twoFactorActive
    ? t("twoFactorTitle")
    : mode === "register"
      ? t("registerTitle")
      : mode === "reset-password"
        ? t("resetPassword")
        : t("authTitle");
  const description = twoFactorActive
    ? t("twoFactorDescription")
    : mode === "register"
      ? t("registerDescription")
      : mode === "reset-password"
        ? t("resetDescription")
        : t("authDescription");

  return (
    <div className="mt-6 space-y-2 text-left first:mt-0">
      <h1 className="text-2xl font-semibold leading-8 tracking-[-0.02em] text-foreground">
        {title}
      </h1>
      <p className="text-sm leading-6 text-muted-foreground">
        {description}
      </p>
    </div>
  );
}

function LoginModeTabs({
  t,
  mode,
  onModeChange,
}: {
  t: LoginMessages;
  mode: LoginMode;
  onModeChange: (mode: "login" | "register") => void;
}) {
  return (
    <div
      className="grid grid-cols-2 gap-1 rounded-xl border border-border/60 bg-muted/70 p-1"
      role="tablist"
      aria-label={t("title")}
    >
      {(["login", "register"] as const).map((item) => {
        const selected = mode === item;
        return (
          <button
            key={item}
            type="button"
            role="tab"
            aria-selected={selected}
            className={cn(
              "relative min-h-11 rounded-lg border px-3 text-sm font-semibold outline-none transition-colors focus-visible:ring-[3px] focus-visible:ring-ring/50 motion-reduce:transition-none",
              selected
                ? "border-primary/45 bg-primary/15 text-foreground shadow-sm shadow-foreground/5 after:absolute after:inset-x-8 after:bottom-1 after:h-0.5 after:rounded-full after:bg-primary after:content-[''] dark:border-primary/65 dark:bg-primary/20"
                : "border-transparent text-muted-foreground hover:bg-background/35 hover:text-foreground",
            )}
            onClick={() => onModeChange(item)}
          >
            {item === "login" ? t("signIn") : t("register")}
          </button>
        );
      })}
    </div>
  );
}

function LoginCardSkeleton() {
  return (
    <div className="min-h-[29rem] animate-pulse space-y-6 motion-reduce:animate-none" aria-hidden>
      <div className="grid grid-cols-2 gap-2 rounded-xl bg-muted/65 p-1">
        <span className="h-11 rounded-lg bg-background/75" />
        <span className="h-11 rounded-lg bg-muted" />
      </div>
      <div className="space-y-3">
        <span className="block h-7 w-2/5 rounded-md bg-muted" />
        <span className="block h-4 w-4/5 rounded-md bg-muted" />
      </div>
      <div className="space-y-4">
        <span className="block h-16 rounded-xl bg-muted" />
        <span className="block h-16 rounded-xl bg-muted" />
        <span className="block h-11 rounded-xl bg-primary/20" />
      </div>
      <span className="block h-px bg-border" />
      <span className="block h-11 rounded-xl bg-muted" />
    </div>
  );
}

export function LoginPage({ nextPath }: LoginPageProps) {
  const t = useTranslations("login");
  const reduceMotion = useReducedMotion() ?? false;
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
                  className="h-11 min-w-0 border-input/70 bg-background/70 text-sm"
                  placeholder={twoFactorUsesEmail ? t("verificationCodePlaceholder") : t("twoFactorPlaceholder")}
                  value={twoFactorCode}
                  onChange={(event) => setTwoFactorCode(event.target.value)}
                  required
                />
                {twoFactorUsesEmail ? (
                  <Button
                    type="button"
                    variant="secondary"
                    className="h-11 min-w-[4.5rem] shrink-0 rounded-lg border border-border/50 bg-muted/60 px-3 text-sm font-semibold text-foreground shadow-none hover:bg-muted"
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
              className="mt-1 h-11 w-full rounded-lg bg-primary text-sm font-semibold text-primary-foreground shadow-none hover:bg-primary/90"
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
              className="h-11 border-input/70 bg-background/70 text-sm"
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
              className="h-11 border-input/70 bg-background/70 text-sm"
              placeholder={t("password")}
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              required
            />
          </div>
          <Button
            className="mt-1 h-11 w-full rounded-lg bg-primary text-sm font-semibold text-primary-foreground shadow-none hover:bg-primary/90"
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
                className="h-11 border-input/70 bg-background/70 text-sm"
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
                className="h-11 border-input/70 bg-background/70 text-sm"
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
                  className="h-11 border-input/70 bg-background/70 text-sm"
                  placeholder={t("verificationCodePlaceholder")}
                  value={resetCode}
                  onChange={(event) => setResetCode(event.target.value)}
                  required
                />
                <Button
                  type="button"
                  variant="secondary"
                  className="h-11 min-w-[4.5rem] shrink-0 rounded-lg border border-border/50 bg-muted/60 px-3 text-sm font-semibold text-foreground shadow-none hover:bg-muted"
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
              className="mt-1 h-11 w-full rounded-lg bg-primary text-sm font-semibold text-primary-foreground shadow-none hover:bg-primary/90"
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
              className="h-11 border-input/70 bg-background/70 text-sm"
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
              className="h-11 border-input/70 bg-background/70 text-sm"
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
                  className="h-11 border-input/70 bg-background/70 text-sm"
                  placeholder={t("verificationCodePlaceholder")}
                  value={registerCode}
                  onChange={(event) => setRegisterCode(event.target.value)}
                  required
                />
                <Button
                  type="button"
                  variant="secondary"
                  className="h-11 min-w-[4.5rem] shrink-0 rounded-lg border border-border/50 bg-muted/60 px-3 text-sm font-semibold text-foreground shadow-none hover:bg-muted"
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
            className="mt-1 h-11 w-full rounded-lg bg-primary text-sm font-semibold text-primary-foreground shadow-none hover:bg-primary/90"
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
                className="h-11 w-full rounded-lg border border-border/50 bg-muted/60 text-sm font-semibold text-foreground shadow-none hover:bg-muted"
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

    </>
  );

  return (
    <PublicBrandSurface>
      <main
        className="h-svh overflow-x-hidden overflow-y-auto bg-background text-foreground"
        style={{ backgroundImage: AMBIENT_BACKGROUND }}
        aria-busy={!configReady}
      >
        <PublicPageHeader showLoginAction={false} />
        <section className="mx-auto flex min-h-[calc(100svh-4rem)] w-full max-w-[1180px] items-start justify-center px-4 py-8 sm:items-center sm:px-6 sm:py-12 lg:px-8 xl:px-0">
          <div className="w-full max-w-[420px] rounded-[1.5rem] border border-border/70 bg-card/88 p-5 shadow-[0_28px_90px_-58px_color-mix(in_oklch,var(--foreground)_45%,transparent)] backdrop-blur-sm sm:p-8">
            {!configReady ? (
              <LoginCardSkeleton />
            ) : (
              <>
                {canShowRegisterSwitch && mode !== "reset-password" && !twoFactorChallengeToken ? (
                  <LoginModeTabs
                    t={t}
                    mode={mode}
                    onModeChange={(nextMode) => loginPage.setMode(nextMode)}
                  />
                ) : null}
                <LoginAuthPanelHeader
                  t={t}
                  mode={mode}
                  twoFactorActive={Boolean(twoFactorChallengeToken)}
                />

                {reduceMotion ? (
                  <div key={modeKey}>{modeContent}</div>
                ) : (
                  <motion.div
                    key={modeKey}
                    initial={{ opacity: 0, y: 6 }}
                    animate={{ opacity: 1, y: 0 }}
                    transition={{ duration: 0.2, ease: ENTRANCE_EASE }}
                  >
                    {modeContent}
                  </motion.div>
                )}
              </>
            )}
          </div>
        </section>
      </main>
    </PublicBrandSurface>
  );
}
