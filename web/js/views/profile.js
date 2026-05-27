import { api } from "../api.js";
import { ensureShell } from "../layout.js";
import { state, setUser } from "../state.js";
import { escapeHTML, attachForm } from "../utils.js";

export async function renderProfile(root, userId) {
    const content = ensureShell(root, `
        <a href="#/feed" class="back-link">← к ленте</a>
        <section id="profile" class="profile placeholder">загрузка профиля…</section>
    `);

    const isMe = state.user && state.user.id === userId;

    try {
        if (isMe) {
            // Own profile — use state (which already includes private fields
            // like email and age that the public endpoint hides).
            renderOwnProfile(content);
        } else {
            const user = await api.get(`/api/users/${encodeURIComponent(userId)}`);
            renderOtherProfile(content, user);
        }
    } catch (err) {
        const sec = content.querySelector("#profile");
        sec.className = "placeholder err";
        sec.textContent = err.status === 404
            ? "пользователь не найден"
            : "не удалось загрузить профиль: " + err.message;
    }
}

// ---- own profile ---------------------------------------------------------

function renderOwnProfile(content) {
    const u = state.user;
    const sec = content.querySelector("#profile");
    sec.className = "profile";
    sec.innerHTML = `
        <header class="profile-head">
            <h2 class="profile-title">Мой профиль</h2>
        </header>
        <form id="profile-form" class="profile-form" novalidate>
            <label>Никнейм
                <input name="nickname" required minlength="3" maxlength="30"
                       value="${escapeHTML(u.nickname || "")}">
            </label>
            <label>Email
                <input value="${escapeHTML(u.email || "")}" readonly class="readonly-field">
            </label>
            <label>Имя
                <input name="firstName" required
                       value="${escapeHTML(u.firstName || "")}">
            </label>
            <label>Фамилия
                <input name="lastName" required
                       value="${escapeHTML(u.lastName || "")}">
            </label>
            <label>Возраст
                <input name="age" type="number" min="13" max="120" required
                       value="${escapeHTML(String(u.age || ""))}">
            </label>
            <label>Пол
                <select name="gender" required>
                    <option value="female" ${u.gender === "female" ? "selected" : ""}>female</option>
                    <option value="male"   ${u.gender === "male"   ? "selected" : ""}>male</option>
                    <option value="other"  ${u.gender === "other"  ? "selected" : ""}>other</option>
                </select>
            </label>

            <fieldset class="password-block">
                <legend>Сменить пароль <span class="muted">(опционально)</span></legend>
                <label>Текущий пароль
                    <input name="currentPassword" type="password" autocomplete="current-password">
                </label>
                <label>Новый пароль
                    <input name="newPassword" type="password" minlength="6" autocomplete="new-password">
                </label>
            </fieldset>

            <p class="form-msg" id="profile-msg"></p>
            <div class="post-form-actions">
                <button type="reset" class="btn-link">Сбросить</button>
                <button type="submit" class="btn-primary">Сохранить</button>
            </div>
        </form>
    `;

    const form = sec.querySelector("#profile-form");
    const msg  = sec.querySelector("#profile-msg");

    attachForm(form, msg, async (fd) => {
        const payload = {
            nickname:  fd.get("nickname"),
            firstName: fd.get("firstName"),
            lastName:  fd.get("lastName"),
            age:       Number(fd.get("age")),
            gender:    fd.get("gender"),
        };
        const newPwd = fd.get("newPassword");
        const curPwd = fd.get("currentPassword");
        if (newPwd) {
            if (!curPwd) throw new Error("укажи текущий пароль чтобы сменить");
            payload.currentPassword = curPwd;
            payload.newPassword = newPwd;
        }

        const updated = await api.put("/api/me", payload);
        setUser(updated);
        msg.textContent = "сохранено";
        msg.classList.add("ok");
        // Clear password fields once they've been used.
        form.querySelector('[name="currentPassword"]').value = "";
        form.querySelector('[name="newPassword"]').value = "";
    });
}

// ---- other user's profile -----------------------------------------------

function renderOtherProfile(content, u) {
    const sec = content.querySelector("#profile");
    sec.className = "profile";
    const joined = new Date(u.createdAt).toLocaleDateString("ru", {
        day: "2-digit", month: "long", year: "numeric",
    });
    sec.innerHTML = `
        <header class="profile-head">
            <h2 class="profile-title">
                <span class="status-dot ${u.online ? "online" : "offline"}"></span>
                ${escapeHTML(u.nickname)}
            </h2>
            <div class="profile-fields">
                <div><span class="profile-field-label">Имя</span>${escapeHTML(u.firstName)}</div>
                <div><span class="profile-field-label">Фамилия</span>${escapeHTML(u.lastName)}</div>
                <div><span class="profile-field-label">Пол</span>${escapeHTML(u.gender)}</div>
                <div><span class="profile-field-label">На форуме</span>с ${escapeHTML(joined)}</div>
            </div>
        </header>
    `;
}
