import { api } from "../api.js";
import { ensureShell } from "../layout.js";
import { state } from "../state.js";
import { navigate } from "../router.js";
import { escapeHTML, timeAgo, attachForm } from "../utils.js";
import { attachReactionHandlers, reactionButtonsHTML } from "../reactions.js";
import { ownerControlsHTML } from "./post-card.js";

export async function renderPost(root, postId) {
    const content = ensureShell(root, `
        <a href="#/feed" class="back-link">← к ленте</a>
        <div id="post-container" class="placeholder">загрузка…</div>
        <section class="comments-section" id="comments-section" hidden>
            <h3 class="comments-title">
                Комментарии <span class="comment-count" id="comment-count"></span>
            </h3>
            <form id="comment-form" class="comment-form" novalidate>
                <textarea
                    name="content"
                    required minlength="1" maxlength="2000"
                    rows="3"
                    placeholder="Написать комментарий…"></textarea>
                <p class="form-msg" id="comment-msg"></p>
                <div class="post-form-actions">
                    <button type="submit" class="btn-primary">Отправить</button>
                </div>
            </form>
            <div id="comment-list" class="comment-list"></div>
        </section>
    `);

    // One delegated reaction handler for the whole content area covers both
    // the post itself and any comment beneath it.
    attachReactionHandlers(content);
    attachOwnerHandlers(content, postId);

    try {
        const [post, comments] = await Promise.all([
            api.get(`/api/posts/${encodeURIComponent(postId)}`),
            api.get(`/api/posts/${encodeURIComponent(postId)}/comments`),
        ]);
        renderPostBody(content, post);
        renderComments(content, comments);
        wireCommentForm(content, postId);
    } catch (err) {
        const c = content.querySelector("#post-container");
        c.className = "placeholder err";
        c.textContent = err.status === 404
            ? "пост не найден"
            : "не удалось загрузить пост: " + err.message;
    }
}

function renderPostBody(content, p) {
    const container = content.querySelector("#post-container");
    container.className = "post post-detail";
    container.dataset.targetKind = "posts";
    container.dataset.targetId = p.id;
    container.dataset.myReaction = String(p.myReaction);

    const tags = (p.categories || [])
        .map(c => `<span class="tag">#${escapeHTML(c)}</span>`)
        .join("");
    const mine = state.user && p.userId === state.user.id;
    const ownerControls = mine ? ownerControlsHTML() : "";
    container.innerHTML = `
        <div class="post-meta-row">
            <a class="user-link" href="#/users/${encodeURIComponent(p.userId)}">${escapeHTML(p.author)}</a>
            <span class="post-meta">· ${timeAgo(p.createdAt)}</span>
            ${ownerControls}
        </div>
        <h2 class="post-title">${escapeHTML(p.title)}</h2>
        ${tags ? `<div class="post-tags">${tags}</div>` : ""}
        <div class="post-content">${escapeHTML(p.content)}</div>
        <footer class="post-actions">
            ${reactionButtonsHTML(p)}
        </footer>
    `;
    content.querySelector("#comments-section").hidden = false;
}

function renderComments(content, comments) {
    const list  = content.querySelector("#comment-list");
    const count = content.querySelector("#comment-count");
    count.textContent = `(${comments.length})`;

    if (!comments.length) {
        list.innerHTML = `<p class="comment-empty">пока ни одного комментария — оставь первый</p>`;
        return;
    }
    list.innerHTML = comments.map(renderCommentCard).join("");
}

function renderCommentCard(c) {
    const mine = state.user && c.userId === state.user.id;
    const ownerControls = mine ? ownerControlsHTML() : "";
    return `
        <article class="comment"
                 data-target-kind="comments"
                 data-target-id="${c.id}"
                 data-my-reaction="${c.myReaction}">
            <header class="comment-head">
                <a class="user-link" href="#/users/${encodeURIComponent(c.userId)}">${escapeHTML(c.author)}</a>
                <span class="comment-meta">${timeAgo(c.createdAt)}</span>
                ${ownerControls}
            </header>
            <div class="comment-body">${escapeHTML(c.content)}</div>
            <footer class="comment-actions">
                ${reactionButtonsHTML(c)}
            </footer>
        </article>
    `;
}

// Delegated click handler: catches ✎/🗑 on the post itself and on any
// comment beneath it. Comments edit inline (textarea swap); the post takes
// the user back to the feed after deletion.
function attachOwnerHandlers(content, postId) {
    content.addEventListener("click", async (e) => {
        const btn = e.target.closest(".owner-btn");
        if (!btn) return;
        e.preventDefault();
        e.stopPropagation();

        const card = btn.closest("[data-target-kind]");
        if (!card) return;
        const kind   = card.dataset.targetKind;   // "posts" | "comments"
        const id     = card.dataset.targetId;
        const action = btn.dataset.action;

        if (kind === "posts" && action === "delete") {
            if (!confirm("Удалить пост?")) return;
            try {
                await api.del(`/api/posts/${encodeURIComponent(id)}`);
                navigate("#/feed");
            } catch (err) {
                alert("не удалось удалить: " + err.message);
            }
            return;
        }

        if (kind === "posts" && action === "edit") {
            startPostEdit(content, card, id);
            return;
        }

        if (kind === "comments" && action === "delete") {
            if (!confirm("Удалить комментарий?")) return;
            try {
                await api.del(`/api/comments/${encodeURIComponent(id)}`);
                const comments = await api.get(
                    `/api/posts/${encodeURIComponent(postId)}/comments`,
                );
                renderComments(content, comments);
            } catch (err) {
                alert("не удалось удалить: " + err.message);
            }
            return;
        }

        if (kind === "comments" && action === "edit") {
            startCommentEdit(content, card, id, postId);
            return;
        }
    });
}

function startPostEdit(content, card, postId) {
    const title   = card.querySelector(".post-title")?.textContent || "";
    const text    = card.querySelector(".post-content")?.textContent || "";
    const cats    = [...card.querySelectorAll(".post-tags .tag")]
        .map(el => el.textContent.replace(/^#/, ""));
    const originalHTML = card.innerHTML;

    card.innerHTML = `
        <form class="post-edit-form" novalidate>
            <label>Заголовок
                <input name="title" required minlength="3" maxlength="200" value="${escapeHTML(title)}">
            </label>
            <label>Текст
                <textarea name="content" required minlength="1" maxlength="5000" rows="5">${escapeHTML(text)}</textarea>
            </label>
            <p class="form-msg"></p>
            <div class="post-form-actions">
                <button type="button" class="btn-link" data-edit-cancel>Отмена</button>
                <button type="submit" class="btn-primary">Сохранить</button>
            </div>
        </form>
    `;

    const form = card.querySelector(".post-edit-form");
    const msg  = card.querySelector(".form-msg");

    card.querySelector("[data-edit-cancel]").addEventListener("click", () => {
        card.innerHTML = originalHTML;
    });

    api.get("/api/categories").then(allCats => {
        const ids = allCats
            .filter(c => cats.includes(c.name))
            .map(c => c.id);
        form.dataset.categoryIds = ids.join(",");
    }).catch(() => {
        form.dataset.categoryIds = "";
    });

    form.addEventListener("submit", async (e) => {
        e.preventDefault();
        msg.textContent = "";
        const fd = new FormData(form);
        const payload = {
            title: fd.get("title"),
            content: fd.get("content"),
            categoryIds: (form.dataset.categoryIds || "")
                .split(",").filter(Boolean).map(Number),
        };
        try {
            await api.put(`/api/posts/${encodeURIComponent(postId)}`, payload);
            // Re-fetch and re-render the whole post (also reflects new content).
            const fresh = await api.get(`/api/posts/${encodeURIComponent(postId)}`);
            renderPostBody(content, fresh);
        } catch (err) {
            msg.textContent = err.message;
            msg.classList.add("err");
        }
    });
}

function startCommentEdit(content, card, commentId, postId) {
    const body = card.querySelector(".comment-body");
    const originalText = body.textContent || "";
    const originalHTML = body.innerHTML;

    body.innerHTML = `
        <form class="comment-edit-form" novalidate>
            <textarea name="content" required minlength="1" maxlength="2000" rows="3">${escapeHTML(originalText)}</textarea>
            <p class="form-msg"></p>
            <div class="post-form-actions">
                <button type="button" class="btn-link" data-edit-cancel>Отмена</button>
                <button type="submit" class="btn-primary">Сохранить</button>
            </div>
        </form>
    `;

    const form = body.querySelector(".comment-edit-form");
    const msg  = body.querySelector(".form-msg");

    body.querySelector("[data-edit-cancel]").addEventListener("click", () => {
        body.innerHTML = originalHTML;
    });

    form.addEventListener("submit", async (e) => {
        e.preventDefault();
        msg.textContent = "";
        const fd = new FormData(form);
        try {
            await api.put(
                `/api/comments/${encodeURIComponent(commentId)}`,
                { content: fd.get("content") },
            );
            const comments = await api.get(
                `/api/posts/${encodeURIComponent(postId)}/comments`,
            );
            renderComments(content, comments);
        } catch (err) {
            msg.textContent = err.message;
            msg.classList.add("err");
        }
    });
}

function wireCommentForm(content, postId) {
    const form = content.querySelector("#comment-form");
    const msg  = content.querySelector("#comment-msg");

    attachForm(form, msg, async (fd) => {
        const data = Object.fromEntries(fd.entries());
        await api.post(
            `/api/posts/${encodeURIComponent(postId)}/comments`,
            data,
        );
        form.reset();
        const comments = await api.get(
            `/api/posts/${encodeURIComponent(postId)}/comments`,
        );
        renderComments(content, comments);
    });
}
