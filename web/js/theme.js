// Theme toggle. The actual data-theme attribute is set in an inline
// <head> script in index.html (to avoid a flash of unstyled content
// on first paint). This module only wires up the toggle button.

const STORAGE_KEY = "theme";

function currentTheme() {
    return document.documentElement.getAttribute("data-theme") || "dark";
}

function applyTheme(theme) {
    document.documentElement.setAttribute("data-theme", theme);
    localStorage.setItem(STORAGE_KEY, theme);
    updateIcon(theme);
}

function updateIcon(theme) {
    const btn = document.getElementById("theme-toggle");
    if (!btn) return;
    // Show the icon of the mode you'll switch INTO.
    btn.textContent = theme === "dark" ? "☀" : "☾";
    btn.setAttribute(
        "aria-label",
        theme === "dark" ? "Включить светлую тему" : "Включить тёмную тему",
    );
}

export function initTheme() {
    const btn = document.getElementById("theme-toggle");
    if (!btn) return;

    updateIcon(currentTheme());

    btn.addEventListener("click", () => {
        applyTheme(currentTheme() === "dark" ? "light" : "dark");
    });
}
