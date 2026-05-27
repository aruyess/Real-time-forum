import { state } from "./state.js";
import { renderRegister } from "./views/register.js";
import { renderLogin } from "./views/login.js";
import { renderFeed } from "./views/feed.js";
import { renderPost } from "./views/post.js";
import { renderChat } from "./views/chat.js";
import { renderProfile } from "./views/profile.js";

// Routes are tried in order. Each pattern can capture groups, which are
// forwarded as positional arguments to the view function.
const routes = [
    { pattern: /^#\/login$/,                 view: renderLogin,    requiresAuth: false },
    { pattern: /^#\/register$/,              view: renderRegister, requiresAuth: false },

    // Feed has four variants. They all reuse renderFeed but pass a different
    // filter so the active item in the sidebar can be highlighted.
    { pattern: /^#\/feed$/,                  view: (r)    => renderFeed(r, null),                requiresAuth: true },
    { pattern: /^#\/feed\/mine$/,            view: (r)    => renderFeed(r, "mine"),              requiresAuth: true },
    { pattern: /^#\/feed\/liked$/,           view: (r)    => renderFeed(r, "liked"),             requiresAuth: true },
    { pattern: /^#\/feed\/c\/([^/]+)$/,      view: (r, c) => renderFeed(r, "category:" + c),     requiresAuth: true },

    { pattern: /^#\/posts\/([^/]+)$/,        view: renderPost,     requiresAuth: true  },
    { pattern: /^#\/chat\/([^/]+)$/,         view: renderChat,     requiresAuth: true  },
    { pattern: /^#\/users\/([^/]+)$/,        view: renderProfile,  requiresAuth: true  },
];

const root = document.getElementById("app");

export function navigate(hash) {
    if (location.hash === hash) {
        render();
    } else {
        location.hash = hash;
    }
}

function findMatch(hash) {
    for (const r of routes) {
        const m = hash.match(r.pattern);
        if (m) return { route: r, params: m.slice(1).map(decodeURIComponent) };
    }
    return null;
}

function render() {
    const hash = location.hash || "#/login";
    const match = findMatch(hash);

    // Unknown route -> sensible default for current auth state.
    if (!match) {
        location.hash = state.user ? "#/feed" : "#/login";
        return;
    }
    // Protected route without a session -> bounce to login.
    if (match.route.requiresAuth && !state.user) {
        location.hash = "#/login";
        return;
    }
    // Already logged in but on a public auth page -> straight to feed.
    if (!match.route.requiresAuth && state.user) {
        location.hash = "#/feed";
        return;
    }
    window.scrollTo(0, 0);
    match.route.view(root, ...match.params);
}

export function startRouter() {
    window.addEventListener("hashchange", render);
    render();
}
