# 主页（/chat 空状态）视觉与排版改进 Plan

> 范围：仅视觉与排版打磨，不新增内容模块。
> 主页构成：`app/page.tsx` → 重定向 `/chat` → `ChatEmptyState`（问候语）+ `ChatInput`（居中输入框），外层为侧边栏布局。
> 设计体系：暖米色背景 + 赭橙主色（oklch 0.6171 0.1375 39），shadcn/Radix + Tailwind 4。

---

## 一、问题诊断

### P1 — 标题衬线字体跨平台不可控（影响最大）

`chat-empty.tsx:46` 的 h1 使用 `--font-economist`：

```
"Iowan Old Style", "Palatino Linotype", Palatino, "Times New Roman", Georgia, "Noto Serif SC", "Songti SC", serif
```

- **Iowan Old Style / Songti SC 仅 macOS 自带**。Windows 用户落到 Palatino Linotype 或 Times New Roman，中文部分落到系统默认 serif（中易宋体观感），Linux/Android 则完全不可预测——同一个主页在三个平台呈现三种气质。
- h1 用 `font-medium`（500），但上述 serif 基本只有 400/700 两档，浏览器会**合成伪 500 字重**，不同平台粗细不一。
- 这是主页唯一的视觉主角，字体不稳定等于品牌气质不稳定。

**方案**：用 `next/font` 自托管一款有 500 字重的衬线字体（拉丁推荐 Source Serif 4 / STIX Two Text；中文配 Noto Serif SC 按需子集化，或仅对拉丁字符用自托管字体、中文保留本地栈但明确降级顺序）。统一后删除各平台 fallback 差异。

### P2 — 输入框容器存在感与 focus 反馈

`chat-input.tsx:422-425`：

- `border-[0.5px] border-border/70`：0.5px 边框在 Windows 标准 DPI 屏上渲染不稳定（可能整段消失或粗细不均）；且 border 色 oklch 87.79% × 70% 透明度叠在 97.86% 的背景上，**对比度极低**，容器几乎只靠 `shadow-xs`（0 1px 3px / 5% 黑）撑起来。
- 基础组件 `input-group.tsx` 本有 focus ring（`border-ring/60 + ring-[1px]`），但这里被显式 `ring-0` 压掉，focus 前后**零视觉变化**。主页上输入框是唯一交互主体，聚焦无反馈会显得"没通电"。

**方案**：
1. 边框改 `border`（1px）+ `border-border/60`，或保留 0.5px 但将 shadow 升级为 `shadow-sm`；
2. focus 态给一个克制的反馈：`focus 时 border-ring/35` 或 shadow 从 xs → sm 的过渡（不需要恢复完整 ring，保持现在的安静气质）。

### P3 — 发送按钮无主次，品牌主色在主页缺席

`chat-input.tsx:793-830`：发送/语音按钮是 ghost 风格 muted 图标，与左侧"+"、配置按钮**视觉权重完全相同**。整个主页上赭橙主色（--primary）几乎不出现。

**方案**：有草稿文字时（`hasDraftText`），发送按钮升级为 **filled 圆形主色按钮**（`bg-primary text-primary-foreground rounded-full`），无文字时保持 ghost 语音图标。一处改动同时解决"主操作不突出"和"主页没有品牌色"两个问题，也符合主流 chat 产品的习惯认知。

### P4 — 标题排版与视觉重心

`chat-empty.tsx`：

- 字号 22px（移动）/ 32px（桌面），相对 800px 宽的输入框偏小，问候语撑不起页面主角的角色。建议 **26px / 38px** 左右，行高维持 1.12，字距可放宽到 -0.01em（衬线大字号下更稳）。
- 外层 `justify-center` 是几何居中，但输入框高（含工具栏 ~130px+），导致**视觉重心偏低**。建议让内容块整体上移：如 `justify-center` 改为居中后加 `-translate-y-[4vh]`，或容器改用 `pb-[12vh]` 类的不对称 padding，让标题落在黄金视线区（约 40-45% 高度）。
- 标题与输入框间距 `mt-7/mt-8` 可以微调到 `mt-9/mt-10`，让标题有独立呼吸感（当前与输入框略显"粘连"）。

### P5 — 暗色模式 token 遗留问题

`globals.css` 默认主题的 `.dark` 块中：

- `--secondary: oklch(0.9245 …)` 与 `--secondary-foreground: oklch(0.4334 …)` **与浅色模式完全相同**——暗色下 secondary 按钮/badge 会是一块亮色（badge.tsx、button.tsx 均有引用）。
- `--sidebar-border: oklch(0.9401 0 0)`（94% 亮度）在暗色侧边栏（背景 23%）上是一条近白色的边线，sidebar.tsx 的分隔线/rail hover 都用它。
- `app/layout.tsx` 的 `viewport.themeColor: "#0f172a"`（slate 蓝黑）与任何主题背景（暖米色/暖灰）都不匹配，PWA/移动端状态栏会出现一条"异色带"。

**方案**：补齐暗色 secondary/sidebar-border 的对应值；themeColor 改为按 media 区分 light/dark 两组值，与 `--background` 对齐。

### P6 — 图标笔画与小细节

- 图标 strokeWidth 混用 1.4 / 1.6 / 1.7 / 1.8 / 2.0（工具栏主图标 1.4，菜单图标 1.6-1.7，关闭 1.8，对勾 2.0），同一视野内粗细不一。建议规范为两档：**工具栏 1.5、菜单/辅助 1.7**。
- 项目模式 badge 用 `absolute left-full` 挂在标题右侧，标题居中时未补偿 badge 宽度，标题组**光学上偏左**。可给标题加对称的 `padding-inline` 或用 `translate-x` 补偿。
- 问候语 10 个随机变体中标点不统一（"随时开始。"句号 vs 其他问句问号），中文语境下建议统一为问句或统一去尾标点。

### P7 — 入场动效缺失

标题切换有 fade/slide 动画，但**首屏没有入场节奏**。motion 已在依赖中，成本很低。

**方案**：首次渲染时标题 → 输入框做 60-80ms 的 stagger fade-up（总时长 ≤ 0.4s，ease 用现有的 `[0.16,1,0.3,1]`），并尊重 `prefers-reduced-motion`。

---

## 二、实施计划

### Phase 1 — 速赢（纯样式，低风险，~半天）

| # | 改动 | 文件 |
|---|------|------|
| 1 | 发送按钮有文字时升级 filled 主色圆形 | `features/chat/components/sections/chat-input.tsx` |
| 2 | 输入框边框/阴影增强 + focus 微反馈 | 同上（`InputGroup` 的 className） |
| 3 | 标题字号 26/38px、字距与间距调整、视觉重心上移 | `features/chat/components/sections/chat-empty.tsx` |
| 4 | viewport themeColor 按明暗双值修正 | `app/layout.tsx` |

### Phase 2 — 字体与 token（~1 天）

| # | 改动 | 文件 |
|---|------|------|
| 5 | next/font 自托管衬线字体替换 `--font-economist` | `app/layout.tsx`、`app/globals.css` |
| 6 | 暗色 `--secondary` / `--sidebar-border` 补齐 | `app/globals.css`（注意 azure 等多主题同步检查） |
| 7 | 图标 strokeWidth 统一两档规范 | `chat-input.tsx` 及相关 section 组件 |

### Phase 3 — 动效与细节（~半天）

| # | 改动 | 文件 |
|---|------|------|
| 8 | 首屏 stagger 入场动画（respect reduced-motion） | `chat-empty.tsx` |
| 9 | 项目 badge 居中光学补偿 | `chat-empty.tsx` |
| 10 | 问候语变体标点统一 | `i18n/messages/*/chat.json` |

### 验证清单

- 三主题（默认暖色 / dark / azure 及其暗色变体）下截屏对比，重点核对 P5 的 token 改动无回归。
- Windows + macOS 双平台核对标题字体渲染（P1 的核心验收点）。
- 断点检查：375px / 768px / 1280px，确认标题缩放与重心上移在小屏不挤压输入框。
- focus / hover / 有草稿 / 发送中四种输入框状态走查。
- 对比度抽查：边框增强后仍需保持"安静"，不要变成强描边卡片。

---

## 三、刻意不做的事

- 不加建议提示词、最近会话等内容模块（本次范围外）。
- 不恢复完整 focus ring——现有"无 ring"是明确的设计取向，只做最小化反馈。
- 不动 `ChatInput` 的功能结构与工具栏布局，只调视觉表层，避免影响会话页复用。
