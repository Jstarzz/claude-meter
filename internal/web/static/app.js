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
    const people = document.getElementById("people");
    const accounts = document.getElementById("accounts");
    const requests = document.getElementById("requests");
    const sectionGrid = document.querySelector(".section-grid");
    const headingTitle = document.querySelector(".heading h1");
    const headingDescription = document.querySelector(".heading p");
    const breadcrumb = document.querySelector(".breadcrumb");
    const baseDocumentTitle = document.title;
    const overviewTitle = headingTitle ? headingTitle.textContent : "Team usage";
    const validViews = new Set(["overview", "people", "accounts", "requests"]);
    const viewCopy = {
      overview: {
        title: overviewTitle,
        description: "Claude Code requests, token usage, accounts, and estimated API-equivalent spend."
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
      if (people) people.hidden = view !== "overview" && view !== "people";
      if (accounts) accounts.hidden = view !== "overview" && view !== "accounts";
      if (requests) requests.hidden = view !== "overview" && view !== "requests";

      if (sectionGrid) {
        sectionGrid.hidden = view === "requests";
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

    renderView(viewFromLocation());
    syncTheme();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init, { once: true });
  } else {
    init();
  }
})();
