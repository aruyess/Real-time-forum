import { api } from "../api.js";
import { setUser } from "../state.js";
import { navigate } from "../router.js";
import { connectWS } from "../ws.js";
import { attachForm } from "../utils.js";

export function renderLogin(root) {
    root.innerHTML = `
        <main class="auth">
            <h1>Вход</h1>
            <form id="login-form" class="auth-form" novalidate>
                <label>Никнейм или email
                    <input name="identifier" required autocomplete="username">
                </label>
                <label>Пароль
                    <input name="password" type="password" required autocomplete="current-password">
                </label>
                <button type="submit">Войти</button>
                <p class="form-msg" id="login-msg"></p>
                <p class="auth-switch">Нет аккаунта? <a href="#/register">Зарегистрироваться</a></p>
            </form>
        </main>
    `;

    const form = root.querySelector("#login-form");
    const msg = root.querySelector("#login-msg");

    attachForm(form, msg, async (fd) => {
        const data = Object.fromEntries(fd.entries());
        const me = await api.post("/api/login", data);
        setUser(me);
        connectWS();
        navigate("#/feed");
    });
}
