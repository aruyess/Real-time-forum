// WebSocket client: single persistent connection per logged-in session.
// Other modules subscribe via onWS() and receive parsed JSON event objects.

let socket = null;
let shouldReconnect = false;
let reconnectDelay = 1000;
const subscribers = new Set();

function emit(data) {
    for (const fn of subscribers) {
        try {
            fn(data);
        } catch (err) {
            console.error("ws subscriber error:", err);
        }
    }
}

function open() {
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    socket = new WebSocket(`${proto}//${location.host}/ws`);

    socket.addEventListener("open", () => {
        reconnectDelay = 1000;
    });

    socket.addEventListener("message", (e) => {
        let data;
        try { data = JSON.parse(e.data); }
        catch (err) { console.warn("ws non-JSON message:", e.data); return; }
        emit(data);
    });

    socket.addEventListener("close", () => {
        socket = null;
        if (shouldReconnect) {
            // Exponential-ish backoff, capped at 15s.
            setTimeout(open, reconnectDelay);
            reconnectDelay = Math.min(reconnectDelay * 2, 15000);
        }
    });

    socket.addEventListener("error", (e) => {
        // The browser already logs WebSocket errors; we just let close handle reconnect.
        console.debug("ws error", e);
    });
}

export function connectWS() {
    if (socket) return;
    shouldReconnect = true;
    open();
}

export function disconnectWS() {
    shouldReconnect = false;
    if (socket) {
        socket.close();
        socket = null;
    }
}

// Subscribe to all incoming events. Returns an unsubscribe function.
export function onWS(fn) {
    subscribers.add(fn);
    return () => subscribers.delete(fn);
}

// Send a JSON message. No-op if not connected.
export function sendWS(data) {
    if (socket && socket.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify(data));
    }
}
