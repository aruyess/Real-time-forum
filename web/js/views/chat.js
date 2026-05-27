import { api } from "../api.js";
import { ensureShell } from "../layout.js";
import { state } from "../state.js";
import { escapeHTML, throttle } from "../utils.js";
import { refreshSidebar } from "../sidebar.js";
import { onWS } from "../ws.js";
import { clear as clearUnread } from "../unread.js";

const PAGE_SIZE = 10;
const SCROLL_THRESHOLD = 80; // px from top before we load the next page
const NEAR_BOTTOM      = 80; // px from bottom for auto-scroll on new msg

// Module-level state: one chat view is mounted at a time. Re-mount tears
// down the previous WS subscription and clears the dedupe set.
let unsubscribe   = null;
let currentOther  = null;
let renderedIDs   = new Set();
// Peer's last_read_at timestamp (ISO string from a chat.read WS event).
// Drives the read-receipt tick on our own outgoing messages.
let peerLastReadAt = null;

// Tell the server "I just opened the chat with peerId; mark everything
// from them as read up to now". Refreshes the sidebar so the unread dot
// goes away, and the peer gets a chat.read WS event for read receipts.
async function markChatRead(peerId) {
    try {
        await api.post(`/api/chats/${encodeURIComponent(peerId)}/read`);
        refreshSidebar();
    } catch (err) {
        // Non-fatal: the dot will just clear on the next /api/users poll.
        console.warn("mark chat read:", err);
    }
}

export async function renderChat(root, otherId) {
    if (unsubscribe) {
        unsubscribe();
        unsubscribe = null;
    }
    renderedIDs = new Set();
    peerLastReadAt = null; // refreshed by chat.read WS events
    currentOther = otherId;
    // Opening this chat is "marking as read" — both locally (in-memory
    // unread map) and on the server (chat_reads table, so the dot doesn't
    // come back after a refresh and the peer sees a read-receipt).
    clearUnread(otherId);
    markChatRead(otherId);

    const content = ensureShell(root, `
        <a href="#/feed" class="back-link">← к ленте</a>
        <section class="chat">
            <header class="chat-head">
                <h3>
                    <a id="chat-with-link" class="user-link" href="#">…</a>
                </h3>
            </header>
            <div class="messages" id="messages">
                <div class="msg-loading" id="msg-loading" hidden>загружаю…</div>
                <div class="msg-empty"   id="msg-empty"   hidden>сообщений пока нет — напиши первым</div>
                <div class="msg-list"    id="msg-list"></div>
            </div>
            <div class="msg-preview" id="msg-preview" hidden></div>
            <form id="msg-form" class="msg-form" novalidate>
                <input type="file" name="image" id="msg-image" accept="image/*" hidden>
                <label for="msg-image" class="msg-attach" title="Прикрепить картинку (до 5 МБ)">📎</label>
                <input
                    name="content"
                    type="text"
                    maxlength="2000"
                    autocomplete="off"
                    placeholder="Написать сообщение…">
                <button type="submit" class="btn-primary">Отправить</button>
            </form>
        </section>
    `);

    // Counterpart header — runs in parallel with the first page of history.
    // Also points the nickname link at the user's profile.
    const headerPromise = api.get(`/api/users/${encodeURIComponent(otherId)}`)
        .then(u => {
            const link = content.querySelector("#chat-with-link");
            link.textContent = u.nickname;
            link.href = `#/users/${encodeURIComponent(otherId)}`;
        })
        .catch(() => {
            content.querySelector("#chat-with-link").textContent = "?";
        });

    const scrollEl = content.querySelector("#messages");
    const list     = content.querySelector("#msg-list");
    const loading  = content.querySelector("#msg-loading");
    const empty    = content.querySelector("#msg-empty");
    const preview  = content.querySelector("#msg-preview");

    let earliest  = null;  // ISO string, anchor for "before"
    let exhausted = false;
    let inflight  = false;

    async function loadOlder() {
        if (inflight || exhausted) return;
        inflight = true;
        loading.hidden = false;
        try {
            const params = new URLSearchParams({
                with:  otherId,
                limit: String(PAGE_SIZE),
            });
            if (earliest) params.set("before", earliest);

            const batch = await api.get(`/api/messages?${params}`);
            if (!batch.length) {
                exhausted = true;
                if (!list.children.length) empty.hidden = false;
                return;
            }
            const fresh = batch.slice().reverse().filter(m => !renderedIDs.has(m.id));
            fresh.forEach(m => renderedIDs.add(m.id));
            const html = fresh.map(renderMessage).join("");

            const prevHeight = scrollEl.scrollHeight;
            const prevTop    = scrollEl.scrollTop;
            list.insertAdjacentHTML("afterbegin", html);

            if (earliest === null) {
                scrollEl.scrollTop = scrollEl.scrollHeight;
            } else {
                scrollEl.scrollTop = prevTop + (scrollEl.scrollHeight - prevHeight);
            }
            earliest = batch[batch.length - 1].createdAt;
            if (batch.length < PAGE_SIZE) exhausted = true;
        } catch (err) {
            empty.hidden = false;
            empty.textContent = "не удалось загрузить: " + err.message;
        } finally {
            inflight = false;
            loading.hidden = true;
        }
    }

    scrollEl.addEventListener("scroll", throttle(() => {
        if (scrollEl.scrollTop < SCROLL_THRESHOLD) loadOlder();
    }, 200));

    // Fetch the peer's last_read_at so initial render shows existing read
    // receipts (otherwise ticks only light up after the next chat.read WS
    // event arrives).
    const readStatePromise = api
        .get(`/api/chats/${encodeURIComponent(otherId)}/read-state`)
        .then(res => { peerLastReadAt = res.peerLastReadAt || null; })
        .catch(() => { peerLastReadAt = null; });

    await Promise.all([headerPromise, loadOlder(), readStatePromise]);
    refreshReadIndicators(list);

    // ---- Real-time: append messages that arrive over WS ----
    unsubscribe = onWS((ev) => {
        // chat.read: peer marked our messages read; flip the ticks on our
        // outgoing bubbles whose createdAt is <= the new read timestamp.
        if (ev.type === "chat.read" && ev.readerId === currentOther) {
            peerLastReadAt = ev.lastReadAt;
            refreshReadIndicators(list);
            return;
        }
        if (ev.type !== "message.new" || !ev.message) return;
        if (!location.hash.startsWith("#/chat/")) return;
        const m = ev.message;
        const self = state.user.id;
        const inThisChat =
            (m.senderId === self          && m.receiverId === currentOther) ||
            (m.senderId === currentOther  && m.receiverId === self);
        if (!inThisChat) return;
        appendIncoming(m, scrollEl, list, empty);

        // If the new message is from the peer and we're actively viewing
        // this chat, push the read marker forward so they get the receipt
        // in real-time too.
        if (m.senderId === currentOther) {
            markChatRead(currentOther);
        }
    });

    // ---- Send form ----
    const form      = content.querySelector("#msg-form");
    const textInput = form.querySelector('[name="content"]');
    const fileInput = form.querySelector("#msg-image");
    let pendingImageUrl = null;

    // Upload the chosen file immediately so the eventual submit is fast and
    // we can show a preview confirming what's attached.
    fileInput.addEventListener("change", async () => {
        const f = fileInput.files[0];
        if (!f) return;
        try {
            const fd = new FormData();
            fd.append("image", f);
            const { url } = await api.upload("/api/uploads/image", fd);
            pendingImageUrl = url;
            preview.innerHTML = `
                <img src="${escapeHTML(url)}" alt="preview">
                <button type="button" id="msg-preview-cancel" class="btn-link">убрать</button>
            `;
            preview.hidden = false;
            preview.querySelector("#msg-preview-cancel").addEventListener("click", clearAttachment);
        } catch (err) {
            textInput.placeholder = "ошибка загрузки: " + err.message;
            fileInput.value = "";
            pendingImageUrl = null;
        }
    });

    function clearAttachment() {
        pendingImageUrl = null;
        fileInput.value = "";
        preview.hidden = true;
        preview.innerHTML = "";
    }

    form.addEventListener("submit", async (e) => {
        e.preventDefault();
        const text = textInput.value.trim();
        if (!text && !pendingImageUrl) return;
        textInput.disabled = true;
        try {
            const msg = await api.post("/api/messages", {
                to: otherId,
                content: text,
                imageUrl: pendingImageUrl || "",
            });
            appendIncoming(msg, scrollEl, list, empty);
            textInput.value = "";
            clearAttachment();
            refreshSidebar();
        } catch (err) {
            textInput.placeholder = "ошибка: " + err.message;
        } finally {
            textInput.disabled = false;
            textInput.focus();
        }
    });
}

function appendIncoming(m, scrollEl, list, empty) {
    if (renderedIDs.has(m.id)) return;
    renderedIDs.add(m.id);
    empty.hidden = true;

    const stickToBottom =
        scrollEl.scrollHeight - scrollEl.scrollTop - scrollEl.clientHeight < NEAR_BOTTOM;

    list.insertAdjacentHTML("beforeend", renderMessage(m));
    if (stickToBottom) scrollEl.scrollTop = scrollEl.scrollHeight;
}

function renderMessage(m) {
    const mine  = m.senderId === state.user.id;
    const stamp = new Date(m.createdAt).toLocaleString("ru", {
        day: "2-digit", month: "2-digit", year: "numeric",
        hour: "2-digit", minute: "2-digit",
    });
    const text  = m.content
        ? `<div class="msg-body">${escapeHTML(m.content)}</div>` : "";
    const img   = m.imageUrl
        ? `<a class="msg-image-link" href="${escapeHTML(m.imageUrl)}" target="_blank" rel="noopener">
               <img class="msg-image" src="${escapeHTML(m.imageUrl)}" alt="attached image">
           </a>` : "";
    // Single tick = delivered; the .read class (toggled by refreshReadIndicators)
    // turns it into a double tick by appending another check via CSS.
    const isRead = mine && peerLastReadAt && new Date(m.createdAt) <= new Date(peerLastReadAt);
    const tick = mine
        ? `<span class="msg-tick ${isRead ? "read" : ""}" title="${isRead ? "прочитано" : "отправлено"}">✓</span>`
        : "";
    return `
        <article class="msg ${mine ? "msg-mine" : "msg-theirs"}" data-created-at="${escapeHTML(m.createdAt)}">
            <header class="msg-head">
                <b>${escapeHTML(m.senderNickname)}</b>
                <span class="msg-time">${escapeHTML(stamp)}</span>
                ${tick}
            </header>
            ${text}
            ${img}
        </article>
    `;
}

// Walk all own outgoing bubbles and flip the read-tick when peerLastReadAt
// is now >= their createdAt. Cheap because the list is small.
function refreshReadIndicators(list) {
    if (!peerLastReadAt) return;
    const readUpTo = new Date(peerLastReadAt);
    list.querySelectorAll(".msg.msg-mine").forEach(el => {
        const ts = el.dataset.createdAt;
        if (!ts) return;
        const tick = el.querySelector(".msg-tick");
        if (!tick) return;
        if (new Date(ts) <= readUpTo) {
            tick.classList.add("read");
            tick.title = "прочитано";
        }
    });
}
