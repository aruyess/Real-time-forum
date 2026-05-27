import { api } from "../api.js";
import { ensureShell } from "../layout.js";
import { escapeHTML, attachForm } from "../utils.js";
import { attachReactionHandlers } from "../reactions.js";
import { renderPostCard } from "./post-card.js";

// renderFeed is invoked by the router with an optional filter string:
//   null               -> all posts
//   "mine"             -> ?author=me
//   "liked"            -> ?liked=me
//   "category:<name>"  -> ?category=<name>
//
// The sidebar is responsible for highlighting the active option based on
// the current location.hash; this view just renders the right posts.

// Module-level cache of the active filter so the post-create form handler
// can reload the right view after submitting (avoids re-parsing the hash).
let activeFilter = null;

export async function renderFeed(root, filter = null) {
    activeFilter = filter;
    const showCreate = filter === "mine";

    const createSection = showCreate ? `
        <section class="post-create">
            <button id="new-post-btn" class="btn-primary">+ Новый пост</button>
            <form id="post-form" class="post-form" hidden novalidate>
                <label>Заголовок
                    <input name="title" required minlength="3" maxlength="200">
                </label>
                <label>Текст
                    <textarea name="content" required minlength="1" maxlength="5000" rows="5"></textarea>
                </label>
                <fieldset class="cat-list">
                    <legend>Категории</legend>
                    <div id="cat-options" class="cat-options">загрузка…</div>
                </fieldset>
                <p class="form-msg" id="post-msg"></p>
                <div class="post-form-actions">
                    <button type="button" id="cancel-post" class="btn-link">Отмена</button>
                    <button type="submit" class="btn-primary">Опубликовать</button>
                </div>
            </form>
        </section>
    ` : "";

    const content = ensureShell(root, `
        <header class="feed-head" id="feed-head"></header>
        ${createSection}
        <section class="post-list" id="post-list">
            <p class="placeholder">загрузка постов…</p>
        </section>
    `);

    renderFeedHead(content, filter);
    attachReactionHandlers(content.querySelector("#post-list"));
    attachPostOwnerHandlers(content);

    if (showCreate) {
        wirePostForm(content);
        await Promise.all([
            loadCategories(content),
            loadPosts(content, queryFor(filter)),
        ]);
    } else {
        await loadPosts(content, queryFor(filter));
    }
}

function renderFeedHead(content, filter) {
    const head = content.querySelector("#feed-head");
    let title;
    if (filter === null)               title = "Лента";
    else if (filter === "mine")        title = "Мои посты";
    else if (filter === "liked")       title = "Понравившиеся";
    else if (filter.startsWith("category:")) {
        title = categoriesFrom(filter).map(c => "#" + c).join(" ");
    } else {
        title = "Лента";
    }
    head.innerHTML = `<h2 class="feed-title">${escapeHTML(title)}</h2>`;
}

function queryFor(filter) {
    if (!filter) return "";
    if (filter === "mine")  return "?author=me";
    if (filter === "liked") return "?liked=me";
    if (filter.startsWith("category:")) {
        const names = categoriesFrom(filter);
        return "?" + names.map(n => `category=${encodeURIComponent(n)}`).join("&");
    }
    return "";
}

// Categories travel through the router as a single comma-joined string
// ("category:news,tech"). Split them back out for rendering and query-building.
function categoriesFrom(filter) {
    return filter.slice("category:".length).split(",").filter(Boolean);
}

// ---------- post creation form ----------

function wirePostForm(content) {
    const newBtn = content.querySelector("#new-post-btn");
    const form   = content.querySelector("#post-form");
    const cancel = content.querySelector("#cancel-post");
    const msg    = content.querySelector("#post-msg");

    newBtn.addEventListener("click", () => {
        newBtn.hidden = true;
        form.hidden = false;
        form.querySelector('[name="title"]').focus();
    });

    cancel.addEventListener("click", () => {
        form.reset();
        msg.textContent = "";
        msg.className = "form-msg";
        form.hidden = true;
        newBtn.hidden = false;
    });

    attachForm(form, msg, async (fd) => {
        const payload = {
            title: fd.get("title"),
            content: fd.get("content"),
            categoryIds: fd.getAll("cat").map(Number),
        };
        await api.post("/api/posts", payload);
        form.reset();
        form.hidden = true;
        newBtn.hidden = false;
        // Reload using whatever filter is currently active.
        await loadPosts(content, queryFor(activeFilter));
    });
}

async function loadCategories(content) {
    const container = content.querySelector("#cat-options");
    try {
        const cats = await api.get("/api/categories");
        container.innerHTML = cats.map(c => `
            <label class="cat-item">
                <input type="checkbox" name="cat" value="${c.id}">
                <span>${escapeHTML(c.name)}</span>
            </label>
        `).join("");
    } catch {
        container.textContent = "не удалось загрузить категории";
    }
}

// ---------- post list ----------

// Delegated click handler for the edit (✎) and delete (🗑) icons on the
// owner's own post cards. attached once when the feed view is built; works
// for any cards re-rendered into #post-list afterwards.
function attachPostOwnerHandlers(content) {
    content.querySelector("#post-list").addEventListener("click", async (e) => {
        const btn = e.target.closest(".owner-btn");
        if (!btn) return;
        e.preventDefault();
        e.stopPropagation();

        const card = btn.closest("[data-target-kind='posts']");
        if (!card) return;
        const postId = card.dataset.targetId;
        const action = btn.dataset.action;

        if (action === "delete") {
            if (!confirm("Удалить пост?")) return;
            try {
                await api.del(`/api/posts/${encodeURIComponent(postId)}`);
                card.remove();
            } catch (err) {
                alert("не удалось удалить: " + err.message);
            }
            return;
        }

        if (action === "edit") {
            startInlineEdit(card, postId);
        }
    });
}

// Swap the post card for an inline edit form (title + content) pre-filled
// with the post's current values. Cancel restores the card; Save calls PUT
// and refreshes the feed.
function startInlineEdit(card, postId) {
    const title   = card.querySelector(".post-title")?.textContent || "";
    const content = card.querySelector(".post-content")?.textContent || "";
    const cats    = [...card.querySelectorAll(".post-tags .tag")]
        .map(el => el.textContent.replace(/^#/, ""));

    const originalHTML = card.innerHTML;

    card.innerHTML = `
        <form class="post-edit-form" novalidate>
            <label>Заголовок
                <input name="title" required minlength="3" maxlength="200" value="${escapeHTML(title)}">
            </label>
            <label>Текст
                <textarea name="content" required minlength="1" maxlength="5000" rows="5">${escapeHTML(content)}</textarea>
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

    // We need the category IDs (not names) to PUT — fetch the catalog once.
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
            // Reload the feed using whatever filter is active.
            const list = card.closest("#post-list");
            const root = list.closest(".content");
            await loadPosts(root, queryFor(activeFilter));
        } catch (err) {
            msg.textContent = err.message;
            msg.classList.add("err");
        }
    });
}

async function loadPosts(content, query = "") {
    const list = content.querySelector("#post-list");
    list.innerHTML = `<p class="placeholder">загрузка…</p>`;
    try {
        const posts = await api.get("/api/posts" + query);
        if (!posts.length) {
            list.innerHTML = `<p class="placeholder">пока пусто — попробуй другой фильтр</p>`;
            return;
        }
        list.innerHTML = posts.map(renderPostCard).join("");
    } catch (err) {
        list.innerHTML = `<p class="placeholder err">не удалось загрузить: ${escapeHTML(err.message)}</p>`;
    }
}
