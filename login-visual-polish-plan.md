# 登录页（/login）视觉与排版改进 Plan

> 范围：仅视觉与排版打磨，不动 `use-login-page.ts` 的功能逻辑，不新增功能入口。
> 页面构成：`app/(auth)/login/login-route.tsx` → `LoginPage`（`features/auth/components/login-page.tsx`）。lg 起两列网格：左列 `LoginLandingCopy`（字标 + 大标题 + 3 个能力点）与 `LoginProductPreview`(模拟模型切换面板），右列认证卡片（密码登录 / 2FA / 注册三态 + 第三方登录）。
> 设计体系：暖米色背景 + 赭橙主色（oklch 0.6171 0.1375 39），shadcn/Radix + Tailwind 4，8 套主题 × 明暗（default/azure/cobalt/graphite/lagoon/ink/ochre/sepia）。
> 关联：与 `homepage-visual-polish-plan.md` 的 P1（品牌衬线字体）、P7（入场动效）共享方案，建议协同落地。

---

## 一、问题诊断

### P1 — 主色被当装饰色滥用，真正的 CTA 反而失焦（影响最大）

整个左列散布着 **11 处** `bg-primary/10 text-primary` 的橙色小元素：

- 能力点序号徽章 ×3（`login-page.tsx:62`）
- 预览面板模型 chips ×4（`:103`，还带 `border-primary/20` 描边）
- 预览条目序号 ×3（`:115`）
- "官方 API" 状态徽章 ×1（`:95`）

而页面唯一的主操作——右侧的登录按钮——也是这个主色。视觉动线被 11 个装饰性橙块打散，**品牌色没有把视线引向行动，而是变成了背景噪声**。

**方案**：装饰元素全面降噪，主色只留给"行动与卖点"：

1. 能力点序号改为中性图标（lucide 线性图标，如 `ArrowLeftRight` / `ImageIcon` / `ShieldCheck`，`text-muted-foreground`），顺带解决 P2 的模式重复；
2. 模型 chips 改 `bg-muted/60 text-foreground border-border/50`——或只高亮其中 1 个表示"当前选中"，更贴近真实产品行为，也让面板有了"状态感"；
3. 预览条目序号改 `bg-muted text-muted-foreground`；
4. 保留主色：登录/注册按钮、"官方 API" 徽章（语义上确实是卖点）。

### P2 — 预览面板"线框图"观感：边框四层嵌套 + 同屏两组编号列表

`LoginProductPreview`（`login-page.tsx:73-137`）的嵌套结构：外卡 `border-border/70` → 内面板 `border-border/60` → 条目 `border-border/50` ×3 → 指标块 `border-border/50` ×3，加上 4 个模型 chips 的描边，**同屏约 14 个描边矩形**。所有元素都用同一套"边框 + 半透明卡底"词汇，面板和右侧表单卡片长得一样重，整页像线框稿。

同时，左列上方的能力点列表（1/2/3 徽章）与面板内条目（01/02/03 徽章）是**同构的视觉模式出现两次**，模板感很强。

底部三个指标块（`:126-133`）字面是"主流/模型、自由/切换、官方/直连"——形容词冒充统计数据，视觉上模仿 stat tiles 却没有数字的说服力，是页面上最弱的一块。

**方案**：

1. **减层**：内面板（`:89`）去边框只留底色差；条目（`:113`）去边框，改用纯底色（`bg-muted/40`）或仅 `divide-y` 分组；描边只保留最外层一处；
2. 能力点按 P1 改图标后，编号视觉只在面板内出现一次；
3. 指标块整体移除（信息密度更高的做法是把"官方 API 直连"并进面板 header 区）。注意：`login-landing-copy.test.mjs` 只断言 i18n key 存在，移除 UI 不破坏测试，但建议同步删 key + 更新测试的 `requiredLandingKeys`，避免留死文案。

### P3 — 全页平铺素色，半透明与 backdrop-blur 没有作用对象

`main` 是纯 `bg-background`，认证卡用 `bg-card/80 backdrop-blur-sm`（`:219`）——但卡片背后是与卡面几乎同亮度的素背景，**半透明和模糊既看不出来也白耗渲染**。整页由文字和边框构成，没有任何视觉锚点，对一个消费级 AI 聊天产品来说观感偏"后台管理系统"。

**方案**（纯 CSS、token 驱动，自动适配 8 主题 × 明暗，不引入图片资源）：

给 `main` 加一层极克制的氛围层，例如右上 + 左下两个大半径 radial 光斑：

```
bg-[radial-gradient(80rem_50rem_at_85%_-10%,--theme(--color-primary/6%),transparent_60%),radial-gradient(60rem_40rem_at_-10%_110%,--theme(--color-primary/4%),transparent_60%)]
```

光斑落在卡片背后，`bg-card/80 + backdrop-blur` 才真正"有东西可透"，页面立刻有了空气感和品牌色温。若不采纳氛围层，则应把卡片改实色 `bg-card` 并去掉 blur——二选一，推荐前者。

### P4 — 大标题 CJK 行高过紧

h1（`:49`）`md:text-[3.25rem] md:leading-[1.03]`、移动端 `leading-[1.08]`。这套行高为拉丁字母设计（en-US 的 "GPT, Gemini, Claude, and Grok in one place" 没问题），但中文/日文是全高方块字：zh-CN 的"GPT、Gemini、Claude、Grok，一站自由切换"在 `max-w-[15ch]` 下必然折行，**52px × 1.03 的两行中文上下行几乎贴住**，ja-JP 同样受影响。

另外 `text-[3.25rem]` 是任意值，不经过 `--ui-font-scale` 缩放（`globals.css:938-953` 只缩放 `--text-*` 体系），与移动端 `text-3xl`（会缩放）行为不一致。

**方案**：行高放宽到 `leading-[1.15]`（md）/ `leading-[1.18]`（移动），拉丁语言如想保留紧排可按 locale 分支；字号换算进 `--text-*` 体系或在 `globals.css` 补一个吃 font-scale 的 utility。

### P5 — 品牌字标双轨：AppLogo 与 .brand-wordmark 两套字体定义

`app-logo.tsx:29` 硬编码 `"Palatino Linotype", Palatino, "Book Antiqua", serif`；而 `globals.css:1236` 的 `.brand-wordmark` 用的是 `--font-economist`（Iowan Old Style 首选、wght 560）。**同一个品牌名在产品内有两套渲染**，且两套都依赖系统字体：Windows 落 Palatino Linotype、macOS 落 Palatino/Iowan、Linux 落任意 serif——登录页是品牌第一触点，跨平台气质完全不可控。

细节：`AppLogo` 的 `width` prop 被接收但从未使用（`login-page.tsx:36` 传了 `width={128}`），是死参数。

**方案**：与主页 Plan P1 合并——`next/font` 自托管衬线字体后，AppLogo 与 `.brand-wordmark` 统一引用同一 font token；清理 `width` 死参数。

### P6 — 认证卡片内部细节

- **内边距偏紧**：420px 宽的主卡片只有 `p-4 sm:p-6`（`:219`），建议 `p-5 sm:p-8`，标题与表单的 `mt-5` 同步放宽，提升"接待感"。
- **两种次级按钮并存**：发送验证码按钮 `border-0 bg-muted`（`:256`、`:399`）vs 第三方登录按钮 `border border-border/50 bg-muted/70`（`:428`），同一卡片内统一为一种。
- **表单与第三方登录之间无分隔**（`:421-422` 只有 `mt-5`）：加一条经典的 `border-t` + "或" 居中分隔线，扫读结构立刻清晰（i18n 加一个 `orContinueWith` key）。
- **注册/登录切换链接无可供性**（`:453-455`）：`font-semibold text-foreground` 没有 hover/underline 状态，看不出可点。加 `underline-offset-4 hover:underline`（或改 `text-primary`）。
- **输入框 resting 边框偏弱**：`border-input/50`（`:246` 等）混合后亮度 ≈87%，落在 98% 的卡面上几乎不可见，Windows 标准 DPI 更明显。focus ring 基础组件已有（`components/ui/input.tsx:12`），不必动；只把 resting 提到 `border-input/70` 左右，保持"安静"但可感知。

### P7 — 移动端首屏看不到表单

DOM 顺序是 文案 → 卡片 → 预览。移动端上字标（h-12）+ 标题 + 描述 + **纵向堆叠的 3 个能力点**（`:56`，mobile 单列）合计约 350px+，375×667 的设备上表单字段被推到首屏以下——而打开 /login 的移动用户意图非常明确：登录。

**方案**：能力点列表在移动端后置或隐藏（如 `hidden sm:grid`，或移到卡片之后渲染一份紧凑版）；标题与卡片间距收紧，确保 667px 高度下账号输入框完整出现在首屏。

### P8 — 表面透明度 7 档并存，多主题下行为不可预期

`bg-card/45`（`:60`）、`dark:bg-card/50`（`:60`）、`bg-card/55`（`:113`）、`bg-card/60`（`:88`）、`dark:bg-card/70`（`:88`）、`bg-card/80`（`:219`）、`bg-background/60`（`:128`）、`bg-background/70`（`:89`、输入框）。在素色背景上这些差异肉眼难辨，纯属维护噪声；而 graphite 主题 `--card` 与 `--background` 同色、azure 的 `--card` 接近纯白，透明度叠加的结果在各主题间完全不可控。

**方案**：收敛为 3 档语义表面并固定——页面氛围层（P3）/ 容器卡 `bg-card`（或统一 /80）/ 嵌入面板 `bg-muted/40`。改动时与 P2 的减层一起做。

### P9 — 动效：有局部、缺整体

`configReady` 的 grid-rows 展开过渡（`:222-227`）已经做得很好；但 ① 首屏标题、能力点、卡片、预览**无入场节奏**；② 登录 ↔ 注册 ↔ 2FA 切换是瞬时替换，卡片高度跳变无过渡。

**方案**：与主页 Plan P7 同一套节奏——首屏 60-80ms stagger fade-up（总时长 ≤0.4s，ease `[0.16,1,0.3,1]`，respect `prefers-reduced-motion`）；模式切换复用现有 grid-rows 技巧做高度过渡 + 内容 fade。

---

## 二、实施计划

### Phase 1 — 速赢（纯类名调整，~半天）

| # | 改动 | 文件 |
|---|------|------|
| 1 | 主色降噪：chips/序号改中性，能力点改图标 | `features/auth/components/login-page.tsx` |
| 2 | 卡片内边距、次级按钮统一、切换链接 hover、输入框 resting 边框 | 同上 |
| 3 | h1 CJK 行高放宽 | 同上 |
| 4 | "或"分隔线（含 3 语言 i18n key） | 同上 + `i18n/messages/*/login.json` |

### Phase 2 — 结构性打磨（~1 天）

| # | 改动 | 文件 |
|---|------|------|
| 5 | 预览面板减层重构 + 指标块移除（同步测试与 i18n key） | `login-page.tsx`、`login-landing-copy.test.mjs`、`i18n/messages/*/login.json` |
| 6 | 背景氛围光斑（或改实色卡，二选一） | `login-page.tsx` |
| 7 | 移动端能力点后置/隐藏，首屏露出表单 | `login-page.tsx` |
| 8 | 表面透明度收敛为 3 档 | `login-page.tsx` |

### Phase 3 — 品牌与动效（~半天，与主页 Plan 协同）

| # | 改动 | 文件 |
|---|------|------|
| 9 | 品牌字体统一（依赖主页 Plan P1 的 next/font 落地） | `shared/components/app-logo.tsx`、`app/globals.css`、`app/layout.tsx` |
| 10 | 首屏 stagger 入场 + 登录/注册切换过渡 | `login-page.tsx` |

### 验证清单

- `node --test features/auth/model/`：`login-page-layout.test.mjs` 断言 `<main>` 必须保留 `h-svh overflow-y-auto overflow-x-hidden`——改背景/布局时不可丢；copy 测试随 #5 同步更新。
- 主题抽查至少：default 明/暗、azure、**graphite**（card 与 background 同色，最能暴露透明度与边框依赖）、sepia 暗色。
- 断点走查：375×667（P7 首屏表单）、768、1024（两列切换点）、1366×768（短屏滚动量）、1920。
- 认证卡四态走查：密码登录 / 2FA（含邮箱验证码倒计时）/ 注册（含 Turnstile）/ 仅第三方登录（`passwordLoginEnabled=false` 时无分隔线场景）。
- zh-CN / en-US / ja-JP 三语标题折行与行高核对。
- `prefers-reduced-motion` 下动效全部退化为直接呈现。

---

## 三、刻意不做的事

- 不动 `use-login-page.ts` 的任何状态逻辑与表单字段结构。
- 不新增"忘记密码"等功能入口（功能范围，不在视觉 Plan 内）。
- 不引入插画、产品截图等图片资源——氛围层全部用 token 驱动的 CSS 渐变，保证 8 主题 × 明暗零成本适配。
- 不重写营销文案语义（删指标块只做减法并同步测试，不新写卖点）。
- 不给输入框加强描边或恢复完整 ring——保持现有"安静"基调，只把 resting 对比度调到可感知。
