import { api } from "./api.js";

// reactions.js — shared helpers for like/dislike buttons on posts and comments.
//
// The DOM contract:
//   - the "card" element (post or comment) carries:
//       data-target-kind="posts" | "comments"
//       data-target-id="<uuid>"
//       data-my-reaction="-1" | "0" | "1"
//   - inside it sit two buttons:
//       <button class="react-btn like"    data-react="like">…<span class="count">N</span></button>
//       <button class="react-btn dislike" data-react="dislike">…<span class="count">N</span></button>
//
// attachReactionHandlers(rootEl) installs a single delegated click handler.

export function attachReactionHandlers(rootEl) {
    rootEl.addEventListener("click", async (e) => {
        const btn = e.target.closest(".react-btn");
        if (!btn) return;
        const card = btn.closest("[data-target-kind][data-target-id]");
        if (!card) return;
        // We may be inside an <a> wrapper (feed cards); cancel its navigation.
        e.preventDefault();
        e.stopPropagation();

        const kind = card.dataset.targetKind;          // posts | comments
        const id   = card.dataset.targetId;
        const cur  = Number(card.dataset.myReaction) || 0;
        const next = btn.dataset.react === "like"
            ? (cur ===  1 ? 0 :  1)
            : (cur === -1 ? 0 : -1);
        try {
            const counts = await api.put(
                `/api/${kind}/${encodeURIComponent(id)}/reactions`,
                { value: next },
            );
            applyReactionCounts(card, counts);
        } catch (err) {
            console.error("reaction failed:", err);
        }
    });
}

// Patch the buttons' counts and active state from a fresh server response.
export function applyReactionCounts(card, counts) {
    card.dataset.myReaction = String(counts.myReaction);
    const like    = card.querySelector('[data-react="like"]');
    const dislike = card.querySelector('[data-react="dislike"]');
    if (like) {
        like.querySelector(".count").textContent = counts.likes;
        like.classList.toggle("active", counts.myReaction === 1);
    }
    if (dislike) {
        dislike.querySelector(".count").textContent = counts.dislikes;
        dislike.classList.toggle("active", counts.myReaction === -1);
    }
}

// reactionButtonsHTML returns the two-button markup ready to drop into a card.
export function reactionButtonsHTML(p) {
    const likeActive    = p.myReaction ===  1 ? "active" : "";
    const dislikeActive = p.myReaction === -1 ? "active" : "";
    return `
        <button type="button" class="react-btn like ${likeActive}" data-react="like" aria-label="лайк">
            <span class="react-icon">👍</span><span class="count">${p.likes}</span>
        </button>
        <button type="button" class="react-btn dislike ${dislikeActive}" data-react="dislike" aria-label="дизлайк">
            <span class="react-icon">👎</span><span class="count">${p.dislikes}</span>
        </button>
    `;
}
