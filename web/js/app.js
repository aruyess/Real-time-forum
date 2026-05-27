import { api } from "./api.js";
import { setUser, state } from "./state.js";
import { startRouter } from "./router.js";
import { initTheme } from "./theme.js";
import { connectWS, onWS } from "./ws.js";
import { increment as bumpUnread } from "./unread.js";

// Bootstrap: wire up the theme toggle, ask the server who we are,
// open the WebSocket if we already have a session, then start the router
// so the first paint resolves to the right view.
async function bootstrap() {
    initTheme();
    try {
        const me = await api.get("/api/me");
        setUser(me);
    } catch {
        // 401 -> not logged in. That's fine, we just start as anonymous.
    }
    if (state.user) {
        connectWS();
        wireUnreadCounter();
    }
    startRouter();
}

// Listen for incoming messages and bump the unread counter for their sender,
// unless the user is already viewing that chat. Lives at app-level so it's
// active regardless of which view is currently rendered.
function wireUnreadCounter() {
    onWS((ev) => {
        if (ev.type !== "message.new" || !ev.message) return;
        const m = ev.message;
        if (m.senderId === state.user.id) return; // our own echo
        const openChat = `#/chat/${encodeURIComponent(m.senderId)}`;
        if (location.hash === openChat) return;   // we're already reading it
        bumpUnread(m.senderId);
    });
}

bootstrap();
