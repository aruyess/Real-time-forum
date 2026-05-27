import { escapeHTML, timeAgo } from "../utils.js";
import { reactionButtonsHTML } from "../reactions.js";
import { state } from "../state.js";

// renderPostCard is shared between the feed and a user profile.
//
// Layout note: the post body is a single clickable `<a>` so the whole card
// opens the detail view. The post metadata (author + time) sits OUTSIDE
// that link so the author can itself be a separate link to the profile
// without nesting <a>'s (which the HTML spec disallows). Reaction buttons
// sit in a footer beneath everything.
export function renderPostCard(p) {
    const tags = (p.categories || [])
        .map(c => `<span class="tag">#${escapeHTML(c)}</span>`)
        .join("");
    const mine = state.user && p.userId === state.user.id;
    const ownerControls = mine ? ownerControlsHTML() : "";
    return `
        <article class="post"
                 data-target-kind="posts"
                 data-target-id="${p.id}"
                 data-my-reaction="${p.myReaction}">
            <div class="post-meta-row">
                <a class="user-link" href="#/users/${encodeURIComponent(p.userId)}">${escapeHTML(p.author)}</a>
                <span class="post-meta">· ${timeAgo(p.createdAt)}</span>
                ${ownerControls}
            </div>
            <a class="post-body-link" href="#/posts/${encodeURIComponent(p.id)}">
                <h3 class="post-title">${escapeHTML(p.title)}</h3>
                ${tags ? `<div class="post-tags">${tags}</div>` : ""}
                <div class="post-content">${escapeHTML(p.content)}</div>
            </a>
            <footer class="post-actions">
                ${reactionButtonsHTML(p)}
            </footer>
        </article>
    `;
}

// Render the small "✎ ✕" icon group used on cards/comments that belong to
// the current user. Click handling lives in feed.js / post.js.
export function ownerControlsHTML() {
    return `
        <span class="owner-actions">
            <button type="button" class="owner-btn edit" data-action="edit"   title="Редактировать">✎</button>
            <button type="button" class="owner-btn del"  data-action="delete" title="Удалить">🗑</button>
        </span>
    `;
}
