// Single source of truth for client-side app state.
// Views read from `state` and call setters; the router decides when to re-render.

export const state = {
    user: null, // { id, nickname, email, firstName, lastName } | null
};

export function setUser(user) {
    state.user = user;
}
