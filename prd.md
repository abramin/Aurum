# PRD: ForeCash (Predictive Personal Finance Workspace)

## 1. Executive Summary

ForeCash is a local-first, highly performant macOS desktop application designed to answer the ultimate financial question: *"If I change my spending today, what does my bank balance look like in six months?"* Inspired by the predictive budgeting tools of Microsoft Money 2005, ForeCash shifts the focus from backward-looking transaction categorization (common in modern apps like MoneyWiz) to forward-looking cash flow visualization. It combines multiple account types, recurring transactions, and flexible budget envelopes to render a real-time, interactive forecast graph.

## 2. Product Principles

### Goals

* **Visual Prediction:** The core experience centers around a forward-looking line graph projecting balances into the future.
* **"What-If" Interactivity:** Users can manipulate sliders (e.g., "increase food budget by $50") and see the graph instantly redraw the compounding effect over months or years.
* **Holistic Wealth View:** Supports multiple account types (Current, Savings, Pensions, Investments) to distinguish between immediate liquid cash flow and long-term net worth.
* **Uncompromising Performance:** Instant load times, zero cloud latency, and rapid data recalculation.
* **Data Privacy:** 100% local data storage. No third-party data harvesting.

### Non-Goals

* **Live Bank Feeds (Plaid, TrueLayer, etc.):** To maintain privacy, avoid subscription costs, and keep the app completely local, data entry will be manual or via CSV import.
* **Complex Double-Entry Accounting:** While it handles transfers accurately, it is not designed to generate formal balance sheets or corporate tax reports.
* **Cloud Syncing & Mobile Apps (For MVP):** The initial focus is a standalone, powerful Mac desktop experience.

## 3. Target Audience & Use Cases

**Primary Persona:** A Mac power user managing personal or household finances who wants strict control over their data and visual clarity on their financial trajectory.

**Core Use Cases:**

* **The Paycheck Check-in:** Viewing the 30-day Liquid Cash graph to ensure the checking account balance doesn't dip below zero before the next payday.
* **Scenario Planning:** Adding a hypothetical one-off expense (e.g., a $2,000 Mac upgrade in 3 months) to instantly see its impact on future cash flow.
* **Budget Calibration:** Tweaking the "Dining Out" envelope slider down by $100/month and watching the 1-year Net Worth line angle upwards.
* **Retirement / Investment Forecasting:** Seeing how a $200 increase in monthly pension transfers compounds over 5 years based on an estimated growth rate.

---

## 4. Core Features & Mechanics

### 4.1 Account Management

The app supports multiple accounts, divided into two primary categories that dictate how they behave in the forecasting engine:

* **Liquid Accounts (Current, Credit Cards, Easy-Access Savings):** Used to calculate the **Cash Flow View**. This is money available to pay immediate bills.
* **Illiquid/Growth Accounts (Pensions, Investments, Fixed-Term Savings):** Included only in the **Net Worth View**. These accounts can have an attached Annual Percentage Rate (APR) to project long-term compounding growth.

### 4.2 The Forecasting Engine

A high-speed calculator that generates daily data points for the graph. It calculates a future day's balance by combining:

1. *Today's Actual Balance*
2. *+ Scheduled Income* (e.g., Salary)
3. *- Scheduled Bills* (e.g., Rent, Utilities)
4. *+/- Scheduled Transfers* (e.g., moving $500 from Current to Savings)
5. *- Prorated Budget Burn* (e.g., a $600/mo food budget reduces the projected daily liquid balance by ~$20/day).

### 4.3 Interactive "What-If" Dashboard

* **The Graph:** A massive, smooth line chart dominating the UI (powered by Apache ECharts). Time horizon toggles between 1M, 3M, 6M, 1Y, and 5Y.
* **The Controls:** A control panel of sliders representing monthly budget categories. Moving a slider sends an immediate instruction to the backend to recalculate the projection array without permanently saving the hypothetical data to the database.

---

## 5. Technical Architecture

To achieve native Mac desktop feel with heavy interactive visualizations, we will use a hybrid stack:

* **Application Framework: Tauri**
* Significantly lighter and faster than Electron.
* Uses the Mac's native WebKit webview for UI rendering, resulting in a tiny app footprint.


* **Backend / Core Logic: Rust**
* Handles database connections, file I/O (CSV parsing), and the heavy mathematical loops required by the Forecasting Engine. Rust guarantees memory safety and blistering speed.


* **Database: SQLite**
* Embedded, zero-configuration local database. Exceptionally fast for the query profiles needed here.


* **Frontend UI: React (TypeScript) + Tailwind CSS**
* Provides a responsive, component-driven interface.


* **Charting Library: Apache ECharts**
* Chosen for its high performance with large datasets (rendering thousands of daily data points without stuttering) and out-of-the-box support for interactive features (like data zoom and drag-to-recalculate).



---

## 6. Data Model (SQLite Schema)

| Table | Core Columns | Description |
| --- | --- | --- |
| **`accounts`** | `id`, `name`, `type`, `balance`, `is_liquid`, `growth_rate_apr` | Stores the baseline starting points and rules for projection. |
| **`transactions`** | `id`, `account_id`, `amount`, `date`, `payee`, `category`, `linked_transfer_id` | Historical ledger. Transfers between accounts link two rows together. |
| **`scheduled_items`** | `id`, `account_id`, `amount`, `frequency`, `next_date`, `type`, `target_account_id` | Fixed inputs for the engine (Income, Bills, Transfers). `frequency` handles recurring logic (weekly, monthly, etc.). |
| **`budgets`** | `id`, `category`, `monthly_limit` | Variable inputs for the engine. Determines the daily "burn rate" of liquid cash. |

---

## 7. UI / UX Layout

1. **Sidebar (Left):** * Global View Toggles (Cash Flow vs. Net Worth).
* List of Accounts with their current actual balances.
* Navigation (Dashboard, Transactions, Scheduled, Settings).


2. **Main Stage (Top):** * The ECharts forecast graph. X-axis is time; Y-axis is balance. Includes a "Today" vertical line. Past is a solid line (actuals); Future is a dashed line (projected).
3. **Main Stage (Bottom):** * The "What-If" Control Center. Sliders for budget categories. Toggles to quickly turn scheduled bills on/off to see their impact.

---

## 8. Development Phases

* **Phase 1 (Data & Engine):** Initialize Tauri/Rust/React skeleton. Set up SQLite schema. Write the Rust calculation engine to combine accounts, scheduled items, and budgets into a future data array.
* **Phase 2 (Visualization):** Implement ECharts in React. Bind the chart to the Rust backend output. Add the interactive "What-If" sliders.
* **Phase 3 (CRUD & UI):** Build the screens to add/edit/delete Accounts, Transactions, and Scheduled Items.
* **Phase 4 (Polish):** CSV import logic for historical transactions, macOS app icon, release build optimizations.
