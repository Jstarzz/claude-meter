(function () {
  const root = document.documentElement;
  let savedTheme = "";

  try {
    savedTheme = localStorage.getItem("claude-meter-theme") || "";
  } catch (_) {}

  const systemTheme = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
  root.dataset.theme = savedTheme === "dark" || savedTheme === "light" ? savedTheme : systemTheme;

  function init() {
    const themeToggle = document.querySelector("[data-theme-toggle]");
    const themeLabel = document.querySelector("[data-theme-label]");
    const refreshButton = document.querySelector("[data-refresh]");
    const navLinks = Array.from(document.querySelectorAll('.side-nav a[href^="#"]'));
    const summary = document.getElementById("overview");
    const efficiency = document.getElementById("efficiency");
    const people = document.getElementById("people");
    const accounts = document.getElementById("accounts");
    const requests = document.getElementById("requests");
    const sectionGrid = document.querySelector(".section-grid");
    const headingTitle = document.querySelector(".heading h1");
    const headingDescription = document.querySelector(".heading p");
    const breadcrumb = document.querySelector(".breadcrumb");
    const baseDocumentTitle = document.title;
    const overviewTitle = headingTitle ? headingTitle.textContent : "Team usage";
    const validViews = new Set(["overview", "efficiency", "people", "accounts", "requests"]);
    const viewCopy = {
      overview: {
        title: overviewTitle,
        description: "Claude Code requests, token usage, accounts, and estimated API-equivalent spend."
      },
      efficiency: {
        title: "Efficiency",
        description: "Measure cache reuse, context churn, tokens per request, and API-equivalent cost efficiency."
      },
      people: {
        title: "People",
        description: "Compare attributed Claude Code usage across developers and devices."
      },
      accounts: {
        title: "Claude accounts",
        description: "See which Claude OAuth accounts generated usage and how that usage breaks down."
      },
      requests: {
        title: "API requests",
        description: "Inspect individual Claude Code API requests, models, tokens, latency, sessions, and spend."
      }
    };

    function syncTheme() {
      const dark = root.dataset.theme === "dark";
      if (themeToggle) {
        themeToggle.setAttribute("aria-pressed", dark ? "true" : "false");
        themeToggle.setAttribute("aria-label", dark ? "Switch to light mode" : "Switch to dark mode");
      }
      if (themeLabel) {
        themeLabel.textContent = dark ? "Light" : "Dark";
      }
    }

    function formatTokens(value) {
      if (!Number.isFinite(value) || value <= 0) return "0";
      if (value >= 1e9) return (value / 1e9).toFixed(2) + "B";
      if (value >= 1e6) return (value / 1e6).toFixed(2) + "M";
      if (value >= 1e3) return (value / 1e3).toFixed(1) + "K";
      return Math.round(value).toString();
    }

    function formatUSD(value) {
      if (!Number.isFinite(value) || value <= 0) return "$0.00";
      return "$" + value.toFixed(value < 1 ? 3 : 2);
    }

    function setEfficiencyValue(name, value) {
      if (!efficiency) return;
      const node = efficiency.querySelector('[data-efficiency-value="' + name + '"]');
      if (node) node.textContent = value;
    }

    function renderEfficiency() {
      if (!efficiency) return;

      const requestCount = Number(efficiency.dataset.requests) || 0;
      const inputTokens = Number(efficiency.dataset.input) || 0;
      const outputTokens = Number(efficiency.dataset.output) || 0;
      const cacheReadTokens = Number(efficiency.dataset.cacheRead) || 0;
      const cacheWriteTokens = Number(efficiency.dataset.cacheWrite) || 0;
      const costMicros = Number(efficiency.dataset.costMicros) || 0;
      const cacheDenominator = cacheReadTokens + cacheWriteTokens + inputTokens;
      const newContextTokens = cacheWriteTokens + inputTokens;
      const cacheReuseRate = cacheDenominator > 0 ? (cacheReadTokens / cacheDenominator) * 100 : 0;
      const reuseMultiple = newContextTokens > 0 ? cacheReadTokens / newContextTokens : 0;
      const cachedPerRequest = requestCount > 0 ? cacheReadTokens / requestCount : 0;
      const inputPerRequest = requestCount > 0 ? inputTokens / requestCount : 0;
      const outputPerRequest = requestCount > 0 ? outputTokens / requestCount : 0;
      const spendPerRequest = requestCount > 0 ? costMicros / 1e6 / requestCount : 0;

      setEfficiencyValue("cache-hit", cacheReuseRate.toFixed(2) + "%");
      setEfficiencyValue("reuse-multiple", reuseMultiple.toFixed(reuseMultiple >= 100 ? 0 : 1) + "x");
      setEfficiencyValue("cached-per-request", formatTokens(cachedPerRequest));
      setEfficiencyValue("input-per-request", formatTokens(inputPerRequest));
      setEfficiencyValue("output-per-request", formatTokens(outputPerRequest));
      setEfficiencyValue("spend-per-request", formatUSD(spendPerRequest));

      const copy = efficiency.querySelector("[data-efficiency-copy]");
      if (!copy) return;
      if (requestCount === 0) {
        copy.textContent = "No API requests in this range yet.";
      } else if (cacheReuseRate >= 99) {
        copy.textContent = "Excellent cache reuse. Keep useful sessions warm and clear at task boundaries, not just because a chat is long. Cache reuse proxy = read / (read + write + uncached input).";
      } else if (cacheReuseRate >= 95) {
        copy.textContent = "Strong cache reuse. Long sessions are paying off; clear when context becomes stale or the task changes. Cache reuse proxy = read / (read + write + uncached input).";
      } else if (cacheReuseRate >= 85) {
        copy.textContent = "Good cache reuse. Watch cache writes and fresh input for signs that context is being rebuilt too often. Cache reuse proxy = read / (read + write + uncached input).";
      } else if (cacheReuseRate >= 60) {
        copy.textContent = "Mixed cache reuse. Session churn, changing prompt prefixes, or frequent clears may be rebuilding context. Cache reuse proxy = read / (read + write + uncached input).";
      } else {
        copy.textContent = "Low cache reuse. Check for short-lived sessions, frequent clears, or unstable prompt prefixes. Cache reuse proxy = read / (read + write + uncached input).";
      }
    }

    function viewFromLocation() {
      const value = window.location.hash.slice(1).toLowerCase();
      return validViews.has(value) ? value : "overview";
    }

    function setActiveNav(view) {
      for (const link of navLinks) {
        const active = link.hash === "#" + view;
        link.classList.toggle("active", active);
        if (active) {
          link.setAttribute("aria-current", "page");
        } else {
          link.removeAttribute("aria-current");
        }
      }
    }

    function preserveViewInLinks(view) {
      const hash = view === "overview" ? "" : "#" + view;
      const links = document.querySelectorAll(".range-tabs a, .leader-row, .person-filter");
      for (const link of links) {
        const href = link.getAttribute("href");
        if (!href) continue;
        const url = new URL(href, window.location.href);
        url.hash = hash;
        link.setAttribute("href", url.pathname + url.search + url.hash);
      }
    }

    function renderView(view) {
      if (!validViews.has(view)) view = "overview";

      if (summary) summary.hidden = view !== "overview";
      if (efficiency) efficiency.hidden = view !== "overview" && view !== "efficiency";
      if (people) people.hidden = view !== "overview" && view !== "people";
      if (accounts) accounts.hidden = view !== "overview" && view !== "accounts";
      if (requests) requests.hidden = view !== "overview" && view !== "requests";

      if (sectionGrid) {
        sectionGrid.hidden = view === "efficiency" || view === "requests";
        sectionGrid.style.gridTemplateColumns = view === "overview" ? "" : "minmax(0, 1fr)";
      }

      const copy = viewCopy[view];
      if (headingTitle) headingTitle.textContent = copy.title;
      if (headingDescription) headingDescription.textContent = copy.description;
      if (breadcrumb) breadcrumb.textContent = "Usage / " + view.charAt(0).toUpperCase() + view.slice(1);
      document.title = view === "overview" ? baseDocumentTitle : copy.title + " | " + baseDocumentTitle;

      setActiveNav(view);
      preserveViewInLinks(view);
    }

    function navigateToView(view) {
      const url = new URL(window.location.href);
      url.hash = view === "overview" ? "" : view;
      history.pushState({ view: view }, "", url);
      renderView(view);
      window.scrollTo({ top: 0, behavior: "smooth" });
    }

    if (themeToggle) {
      themeToggle.addEventListener("click", function () {
        root.dataset.theme = root.dataset.theme === "dark" ? "light" : "dark";
        try {
          localStorage.setItem("claude-meter-theme", root.dataset.theme);
        } catch (_) {}
        syncTheme();
      });
    }

    if (refreshButton) {
      refreshButton.addEventListener("click", function () {
        window.location.reload();
      });
    }

    for (const link of navLinks) {
      link.addEventListener("click", function (event) {
        event.preventDefault();
        navigateToView(link.hash.slice(1));
      });
    }

    window.addEventListener("popstate", function () {
      renderView(viewFromLocation());
    });

    window.addEventListener("hashchange", function () {
      renderView(viewFromLocation());
    });

    renderEfficiency();
    renderView(viewFromLocation());
    syncTheme();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init, { once: true });
  } else {
    init();
  }
})();
