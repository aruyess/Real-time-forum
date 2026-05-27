import { api } from "./api.js";
import { state, setUser } from "./state.js";
import { navigate } from "./router.js";
import { escapeHTML } from "./utils.js";
import { renderSidebar, avatarColor, initial } from "./sidebar.js";
import { disconnectWS } from "./ws.js";

// Idempotent: only renders the shell (topbar + sidebar + content) the first
// time. On subsequent route changes we keep the shell intact and just return
// the existing content element — this preserves the sidebar's state and its
// active WebSocket subscription across views.
export function ensureShell(root, contentHTML = "") {
    const existing = root.querySelector("#content");
    if (existing) {
        existing.innerHTML = contentHTML;
        return existing;
    }

    const nick = state.user.nickname;
    root.innerHTML = `
        <header class="topbar">
            <a class="brand" href="#/feed">
                <span class="brand-mark">F</span>
            </a>
            <div class="topbar-user">
                <a class="user-chip" href="#/users/${encodeURIComponent(state.user.id)}">
                    <span class="avatar" style="background:${avatarColor(nick)}">${initial(nick)}</span>
                    <span class="user-chip-name">${escapeHTML(nick)}</span>
                </a>
                <button id="logout-btn" class="btn-link">Выйти</button>
            </div>
        </header>
        <div class="layout">
            <aside class="sidebar" id="sidebar"></aside>
            <main class="content" id="content">${contentHTML}</main>
        </div>
    `;

    root.querySelector("#logout-btn").addEventListener("click", onLogout);
    renderSidebar(root.querySelector("#sidebar"));
    return root.querySelector("#content");
}

export function teardownShell(root) {
    root.innerHTML = "";
}

async function onLogout() {
    disconnectWS();
    try { await api.post("/api/logout"); } catch { /* drop local session anyway */ }
    setUser(null);
    navigate("#/login");
}
