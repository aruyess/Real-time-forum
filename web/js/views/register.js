import { api } from "../api.js";
import { navigate } from "../router.js";
import { attachForm } from "../utils.js";

export function renderRegister(root) {
    root.innerHTML = `
        <main class="auth">
            <h1>Регистрация</h1>
            <form id="register-form" class="auth-form" novalidate>
                <label>Никнейм
                    <input name="nickname" required minlength="3" maxlength="30">
                </label>
                <label>Email
                    <input name="email" type="email" required>
                </label>
                <label>Пароль
                    <input name="password" type="password" required minlength="6">
                </label>
                <label>Возраст
                    <input name="age" type="number" min="13" max="120" required>
                </label>
                <label>Пол
                    <select name="gender" required>
                        <option value="">—</option>
                        <option value="female">female</option>
                        <option value="male">male</option>
                        <option value="other">other</option>
                    </select>
                </label>
                <label>Имя
                    <input name="firstName" required>
                </label>
                <label>Фамилия
                    <input name="lastName" required>
                </label>
                <button type="submit">Зарегистрироваться</button>
                <p class="form-msg" id="register-msg"></p>
                <p class="auth-switch">Уже есть аккаунт? <a href="#/login">Войти</a></p>
            </form>
        </main>
    `;

    const form = root.querySelector("#register-form");
    const msg = root.querySelector("#register-msg");

    attachForm(form, msg, async (fd) => {
        const data = Object.fromEntries(fd.entries());
        data.age = Number(data.age);
        const result = await api.post("/api/register", data);
        msg.textContent = `Готово! ${result.nickname}, теперь войди.`;
        msg.classList.add("ok");
        setTimeout(() => navigate("#/login"), 800);
    });
}
