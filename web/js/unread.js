// Tracks unread-message counts per peer user.
//
// In-memory only — a page reload resets everything to zero, which matches
// the task's expectation of an ephemeral "you're back, you've caught up"
// model. The proper alternative (server-tracked read state) would require
// extra storage and is out of scope for this project.

const counts = new Map();           // userId -> count
const listeners = new Set();        // () => void

export function get(userId) {
    return counts.get(userId) || 0;
}

export function increment(userId) {
    counts.set(userId, (counts.get(userId) || 0) + 1);
    notify();
}

export function clear(userId) {
    if (counts.delete(userId)) notify();
}

// Subscribe to any count change. Returns an unsubscribe function.
export function onChange(fn) {
    listeners.add(fn);
    return () => listeners.delete(fn);
}

function notify() {
    for (const fn of listeners) {
        try { fn(); } catch (err) { console.error("unread listener:", err); }
    }
}
