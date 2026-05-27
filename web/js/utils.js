// Small frontend helpers reused across views.

// Escape any string before inserting into innerHTML.
export function escapeHTML(s) {
    return String(s).replace(/[&<>"']/g, c => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
    }[c]));
}

// Human-readable relative time, ISO-string in -> short label out.
export function timeAgo(iso) {
    const sec = (Date.now() - new Date(iso).getTime()) / 1000;
    if (sec < 60)     return "только что";
    if (sec < 3600)   return Math.floor(sec / 60)    + " мин назад";
    if (sec < 86400)  return Math.floor(sec / 3600)  + " ч назад";
    if (sec < 604800) return Math.floor(sec / 86400) + " дн назад";
    return new Date(iso).toLocaleDateString();
}

// Throttle: fires at most once per `ms`. The first call is immediate, the
// next can fire no sooner than `ms` later. Useful for scroll events.
export function throttle(fn, ms) {
    let last = 0;
    let pending = null;
    return function (...args) {
        const now = Date.now();
        const since = now - last;
        if (since >= ms) {
            last = now;
            fn.apply(this, args);
        } else if (!pending) {
            // Schedule a trailing call so the very last event isn't lost.
            pending = setTimeout(() => {
                last = Date.now();
                pending = null;
                fn.apply(this, args);
            }, ms - since);
        }
    };
}

// Debounce: defers until `ms` of quiet. Useful when only the final value
// matters (e.g. search-as-you-type).
export function debounce(fn, ms) {
    let timer = null;
    return function (...args) {
        clearTimeout(timer);
        timer = setTimeout(() => fn.apply(this, args), ms);
    };
}

// attachForm wires the standard submit-with-error-message pattern used by
// every form in the app: clear the message paragraph, run the callback,
// and on any thrown error show it in red. `onSubmit(fd)` receives a
// FormData and may return a promise; throw to surface a server error.
export function attachForm(form, msg, onSubmit) {
    form.addEventListener("submit", async (e) => {
        e.preventDefault();
        msg.textContent = "";
        msg.className = "form-msg";
        try {
            await onSubmit(new FormData(form));
        } catch (err) {
            msg.textContent = err.message;
            msg.classList.add("err");
        }
    });
}
