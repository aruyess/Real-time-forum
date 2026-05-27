import { api } from "./api.js";
import { state } from "./state.js";
import { escapeHTML, debounce } from "./utils.js";
import { onWS } from "./ws.js";
import { navigate } from "./router.js";
import { get as unreadFor, onChange as onUnreadChange } from "./unread.js";

// Many message.new events in quick succession would otherwise trigger a
// fresh /api/users fetch each time. Debounce to coalesce bursts.
const debouncedRefresh = debounce(() => refreshSidebar(), 250);

// Module-level — there is one sidebar mounted at a time. Re-mounting tears
// down the previous WS subscription so we don't leak listeners across renders.
let cachedEl = null;
let unsubscribe = null;
let unsubscribeUnread = null;
let hashListenerAttached = false;

export async function renderSidebar(el) {
    cachedEl = el;
    if (unsubscribe) {
        unsubscribe();
        unsubscribe = null;
    }
    if (unsubscribeUnread) {
        unsubscribeUnread();
        unsubscribeUnread = null;
    }

    // Render the static shell first; lists fill in as data arrives.
    el.innerHTML = `
        <nav class="sidebar-section">
            <h3 class="sidebar-head">Навигация</h3>
            <ul class="nav-list">
                <li><a class="nav-item" href="#/feed"       data-route="feed">🏠  Главная</a></li>
                <li><a class="nav-item" href="#/feed/mine"  data-route="mine">📝  Мои посты</a></li>
                <li><a class="nav-item" href="#/feed/liked" data-route="liked">💚  Понравившиеся</a></li>
            </ul>
        </nav>

        <section class="sidebar-section">
            <h3 class="sidebar-head">Категории</h3>
            <ul class="category-list" id="category-list">
                <li class="user-empty">загрузка…</li>
            </ul>
        </section>

        <section class="sidebar-section">
            <h3 class="sidebar-head">Пользователи</h3>
            <div class="sidebar-presence-legend">online · offline</div>
            <div class="presence-group" id="online-group" hidden>
                <h4 class="presence-subhead">
                    <span class="presence-dot online"></span>
                    <span class="presence-count" id="online-count">0</span>
                </h4>
                <ul class="user-list" id="online-list"></ul>
            </div>
            <div class="presence-group" id="offline-group" hidden>
                <h4 class="presence-subhead">
                    <span class="presence-dot offline"></span>
                    <span class="presence-count" id="offline-count">0</span>
                </h4>
                <ul class="user-list" id="offline-list"></ul>
            </div>
            <p class="user-empty" id="presence-empty" hidden>пока никого нет</p>
        </section>
    `;

    // Single event-delegated click handler for the whole sidebar's user rows
    // and the (toggleable) category chips.
    el.addEventListener("click", onUserClick);
    el.addEventListener("click", onCategoryClick);

    if (!hashListenerAttached) {
        window.addEventListener("hashchange", updateActive);
        hashListenerAttached = true;
    }

    try {
        const [users, cats] = await Promise.all([
            api.get("/api/users"),
            api.get("/api/categories"),
        ]);
        renderUserList(el, users);
        renderCategoryList(el, cats);
        updateActive();
    } catch (err) {
        el.querySelector("#presence-empty").textContent = "ошибка: " + err.message;
    }

    unsubscribe = onWS((ev) => {
        if (ev.type === "user.online" || ev.type === "user.offline") {
            if (ev.userId && state.user && ev.userId === state.user.id) return;
            // Status changes can move users between online/offline groups —
            // simplest path is a full re-fetch (debounced).
            debouncedRefresh();
            return;
        }
        if (ev.type === "message.new") {
            debouncedRefresh();
        }
    });

    unsubscribeUnread = onUnreadChange(() => updateBadges());
}

// Re-fetch and re-render the user list. Used after sending a message to
// bump the recipient to the top of the Online group.
export async function refreshSidebar() {
    if (!cachedEl) return;
    try {
        const users = await api.get("/api/users");
        renderUserList(cachedEl, users);
        updateActive();
    } catch {
        // Keep old data on transient failures.
    }
}

// ---- presence group rendering -------------------------------------------

function renderUserList(el, users) {
    const onlineList   = el.querySelector("#online-list");
    const offlineList  = el.querySelector("#offline-list");
    const onlineGroup  = el.querySelector("#online-group");
    const offlineGroup = el.querySelector("#offline-group");
    const onlineCount  = el.querySelector("#online-count");
    const offlineCount = el.querySelector("#offline-count");
    const empty        = el.querySelector("#presence-empty");

    // Always include the logged-in user themselves at the top of the
    // online group — matches the Discord/classmate UX of "I should see
    // myself in the user list".
    const selfRow = state.user
        ? { id: state.user.id, nickname: state.user.nickname, online: true, _self: true }
        : null;

    const allUsers = selfRow ? [selfRow, ...users] : users;
    const online  = allUsers.filter(u => u.online);
    const offline = allUsers.filter(u => !u.online);

    onlineList.innerHTML  = online.map(renderUserItem).join("");
    offlineList.innerHTML = offline.map(renderUserItem).join("");

    onlineCount.textContent  = String(online.length);
    offlineCount.textContent = String(offline.length);

    onlineGroup.hidden  = online.length === 0;
    offlineGroup.hidden = offline.length === 0;
    empty.hidden = !!(online.length || offline.length);
}

function renderUserItem(u) {
    const isSelf = !!u._self;
    // Two sources of "this peer has new messages": the server-tracked
    // hasUnread flag (survives reloads, populated from chat_reads) and the
    // in-memory WS counter bumped while the tab is open. Either is enough
    // to render the dot.
    const hasUnread = !isSelf && (u.hasUnread || unreadFor(u.id) > 0);
    const badge  = hasUnread ? `<span class="unread-dot" title="новое сообщение"></span>` : "";
    const dm     = isSelf ? "" : `<button type="button" class="dm-btn" tabindex="-1" title="Написать">DM</button>`;
    const youTag = isSelf ? `<span class="self-tag">(you)</span>` : "";
    return `
        <li class="user-item ${isSelf ? "user-item-self" : ""}" data-user-id="${u.id}">
            <span class="avatar avatar-sm" style="background:${avatarColor(u.nickname)}">${initial(u.nickname)}</span>
            <span class="user-nick">${escapeHTML(u.nickname)}</span>
            ${youTag}
            ${badge}
            ${dm}
        </li>
    `;
}

// ---- categories rendering -----------------------------------------------

function renderCategoryList(el, cats) {
    const list = el.querySelector("#category-list");
    if (!cats.length) {
        list.innerHTML = `<li class="user-empty">нет категорий</li>`;
        return;
    }
    // Buttons (not <a>'s) because each click toggles membership in a
    // multi-select; the resulting URL is built in onCategoryClick.
    list.innerHTML = cats.map(c => `
        <li>
            <button type="button" class="category-link"
                    data-category="${escapeHTML(c.name)}">#${escapeHTML(c.name)}</button>
        </li>
    `).join("");
}

// Parse the active categories out of the current hash, e.g.
// "#/feed/c/news,tech" -> Set {"news", "tech"}. Returns an empty Set when
// the route isn't a category route.
function selectedCategories() {
    const m = (location.hash || "").match(/^#\/feed\/c\/([^/]+)$/);
    if (!m) return new Set();
    return new Set(
        decodeURIComponent(m[1]).split(",").map(s => s.trim()).filter(Boolean),
    );
}

function onCategoryClick(e) {
    const btn = e.target.closest(".category-link");
    if (!btn) return;
    const name = btn.dataset.category;
    if (!name) return;

    const sel = selectedCategories();
    if (sel.has(name)) sel.delete(name);
    else sel.add(name);

    if (sel.size === 0) {
        navigate("#/feed");
    } else {
        const joined = [...sel].map(encodeURIComponent).join(",");
        navigate(`#/feed/c/${joined}`);
    }
}

// ---- unread badge patcher -----------------------------------------------

function updateBadges() {
    if (!cachedEl) return;
    cachedEl.querySelectorAll(".user-item").forEach(li => {
        const id = li.dataset.userId;
        const count = unreadFor(id);
        let dot = li.querySelector(".unread-dot");
        if (count > 0) {
            if (!dot) {
                dot = document.createElement("span");
                dot.className = "unread-dot";
                dot.title = "новое сообщение";
                li.appendChild(dot);
            }
        } else if (dot) {
            dot.remove();
        }
    });
}

// ---- click + active-state ------------------------------------------------

function onUserClick(e) {
    // Nav anchors are handled natively by the browser; categories are
    // handled in onCategoryClick. Anything else here must be a user row.
    if (e.target.closest("a.nav-item, .category-link")) return;

    const li = e.target.closest(".user-item");
    if (!li) return;
    const id = li.dataset.userId;
    if (!id) return;
    // Clicking your own row in the list opens your profile (you can't DM
    // yourself, and the row has no DM button).
    if (li.classList.contains("user-item-self")) {
        navigate(`#/users/${encodeURIComponent(id)}`);
        return;
    }
    navigate(`#/chat/${encodeURIComponent(id)}`);
}

function updateActive() {
    if (!cachedEl) return;
    const hash = location.hash || "";

    // Navigation items
    const navMap = {
        feed:  "#/feed",
        mine:  "#/feed/mine",
        liked: "#/feed/liked",
    };
    cachedEl.querySelectorAll(".nav-item").forEach(a => {
        a.classList.toggle("active", hash === navMap[a.dataset.route]);
    });

    // Category chips: multi-select, so check membership in the active Set.
    const activeCats = selectedCategories();
    cachedEl.querySelectorAll(".category-link").forEach(btn => {
        btn.classList.toggle("active", activeCats.has(btn.dataset.category));
    });

    // Active chat partner
    const chatMatch = hash.match(/^#\/chat\/([^/]+)$/);
    const activeChat = chatMatch ? decodeURIComponent(chatMatch[1]) : null;
    cachedEl.querySelectorAll(".user-item").forEach(li => {
        li.classList.toggle("active", li.dataset.userId === activeChat);
    });
}

// ---- avatar helpers ------------------------------------------------------

const AVATAR_COLORS = [
    "#7c3aed", "#0ea5e9", "#f59e0b", "#10b981",
    "#ef4444", "#ec4899", "#22c55e", "#3b82f6",
];

function avatarColor(name) {
    let h = 0;
    for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) | 0;
    return AVATAR_COLORS[Math.abs(h) % AVATAR_COLORS.length];
}

function initial(name) {
    return (name[0] || "?").toUpperCase();
}

// Exposed so layout.js can reuse the same color scheme for the topbar avatar.
export { avatarColor, initial };
