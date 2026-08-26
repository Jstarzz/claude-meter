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
    const sections = navLinks
      .map((link) => document.getElementById(link.hash.slice(1)))
      .filter(Boolean);

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

    function setActiveNav(id) {
      for (const link of navLinks) {
        const active = link.hash === "#" + id;
        link.classList.toggle("active", active);
        if (active) {
          link.setAttribute("aria-current", "location");
        } else {
          link.removeAttribute("aria-current");
        }
      }
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
        const target = document.getElementById(link.hash.slice(1));
        if (!target) return;
        event.preventDefault();
        target.scrollIntoView({ behavior: "smooth", block: "start" });
        history.replaceState(null, "", link.hash);
        setActiveNav(target.id);
      });
    }

    if ("IntersectionObserver" in window && sections.length) {
      const observer = new IntersectionObserver(
        function (entries) {
          const visible = entries
            .filter((entry) => entry.isIntersecting)
            .sort((a, b) => b.intersectionRatio - a.intersectionRatio)[0];
          if (visible) setActiveNav(visible.target.id);
        },
        { rootMargin: "-18% 0px -68% 0px", threshold: [0.05, 0.25, 0.5] }
      );
      for (const section of sections) observer.observe(section);
    }

    if (window.location.hash) {
      const initial = document.getElementById(window.location.hash.slice(1));
      if (initial) setActiveNav(initial.id);
    } else {
      setActiveNav("overview");
    }

    syncTheme();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init, { once: true });
  } else {
    init();
  }
})();
