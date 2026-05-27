package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/mail"
	"strings"

	"forum/internal/db"
	"forum/internal/models"
	"forum/internal/session"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type Auth struct {
	DB *sql.DB
}

// ---------- POST /api/register ----------

type registerReq struct {
	Nickname  string `json:"nickname"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Age       int    `json:"age"`
	Gender    string `json:"gender"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
}

func (a *Auth) Register(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Gender = strings.TrimSpace(strings.ToLower(req.Gender))
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)

	if msg := validateRegister(&req); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("bcrypt: %v", err)
		writeError(w, http.StatusInternalServerError, "could not hash password")
		return
	}

	u := &models.User{
		ID:           uuid.NewString(),
		Nickname:     req.Nickname,
		Email:        req.Email,
		PasswordHash: string(hash),
		Age:          req.Age,
		Gender:       req.Gender,
		FirstName:    req.FirstName,
		LastName:     req.LastName,
	}

	if err := db.CreateUser(r.Context(), a.DB, u); err != nil {
		switch {
		case errors.Is(err, db.ErrNicknameTaken):
			writeError(w, http.StatusConflict, "nickname is already taken")
		case errors.Is(err, db.ErrEmailTaken):
			writeError(w, http.StatusConflict, "email is already registered")
		default:
			log.Printf("create user: %v", err)
			writeError(w, http.StatusInternalServerError, "could not create user")
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]string{
		"id":       u.ID,
		"nickname": u.Nickname,
	})
}

func validateRegister(r *registerReq) string {
	if n := len(r.Nickname); n < 3 || n > 30 {
		return "nickname must be 3–30 characters"
	}
	if _, err := mail.ParseAddress(r.Email); err != nil {
		return "invalid email"
	}
	if len(r.Password) < 6 {
		return "password must be at least 6 characters"
	}
	if r.Age < 13 || r.Age > 120 {
		return "age must be between 13 and 120"
	}
	switch r.Gender {
	case "male", "female", "other":
	default:
		return "gender must be male, female, or other"
	}
	if r.FirstName == "" || r.LastName == "" {
		return "first and last name are required"
	}
	return ""
}

// ---------- POST /api/login ----------

type loginReq struct {
	Identifier string `json:"identifier"`
	Password   string `json:"password"`
}

func (a *Auth) Login(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Identifier = strings.TrimSpace(req.Identifier)
	if req.Identifier == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "identifier and password required")
		return
	}

	user, err := db.GetUserByLogin(r.Context(), a.DB, req.Identifier)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		log.Printf("get user by login: %v", err)
		writeError(w, http.StatusInternalServerError, "could not log in")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if _, err := session.Create(r.Context(), a.DB, w, user.ID); err != nil {
		log.Printf("create session: %v", err)
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}

	writeJSON(w, http.StatusOK, userPublic(user))
}

// ---------- GET /api/me ----------

func (a *Auth) Me(w http.ResponseWriter, r *http.Request) {
	s, err := session.FromRequest(r.Context(), a.DB, r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	user, err := db.GetUserByID(r.Context(), a.DB, s.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	writeJSON(w, http.StatusOK, userPublic(user))
}

// ---------- PUT /api/me ----------

// updateMeReq accepts profile fields plus an optional password change.
// If NewPassword is non-empty, CurrentPassword must match the stored hash.
type updateMeReq struct {
	Nickname        string `json:"nickname"`
	FirstName       string `json:"firstName"`
	LastName        string `json:"lastName"`
	Age             int    `json:"age"`
	Gender          string `json:"gender"`
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (a *Auth) UpdateMe(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireAuth(a.DB, w, r)
	if !ok {
		return
	}

	var req updateMeReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.FirstName = strings.TrimSpace(req.FirstName)
	req.LastName = strings.TrimSpace(req.LastName)
	req.Gender = strings.TrimSpace(strings.ToLower(req.Gender))

	if n := len(req.Nickname); n < 3 || n > 30 {
		writeError(w, http.StatusBadRequest, "nickname must be 3–30 characters")
		return
	}
	if req.FirstName == "" || req.LastName == "" {
		writeError(w, http.StatusBadRequest, "first and last name are required")
		return
	}
	if req.Age < 13 || req.Age > 120 {
		writeError(w, http.StatusBadRequest, "age must be between 13 and 120")
		return
	}
	switch req.Gender {
	case "male", "female", "other":
	default:
		writeError(w, http.StatusBadRequest, "gender must be male, female, or other")
		return
	}

	// Optional password change. Triggered only when newPassword is set.
	if req.NewPassword != "" {
		if len(req.NewPassword) < 6 {
			writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}
		current, err := db.GetUserByID(r.Context(), a.DB, userID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "not authenticated")
			return
		}
		if err := bcrypt.CompareHashAndPassword(
			[]byte(current.PasswordHash), []byte(req.CurrentPassword),
		); err != nil {
			writeError(w, http.StatusUnauthorized, "current password is incorrect")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("bcrypt: %v", err)
			writeError(w, http.StatusInternalServerError, "could not change password")
			return
		}
		if err := db.UpdateUserPassword(r.Context(), a.DB, userID, string(hash)); err != nil {
			log.Printf("update password: %v", err)
			writeError(w, http.StatusInternalServerError, "could not change password")
			return
		}
	}

	err := db.UpdateProfile(r.Context(), a.DB, userID, db.ProfileUpdate{
		Nickname:  req.Nickname,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Age:       req.Age,
		Gender:    req.Gender,
	})
	if err != nil {
		switch {
		case errors.Is(err, db.ErrNicknameTaken):
			writeError(w, http.StatusConflict, "nickname is already taken")
		case errors.Is(err, sql.ErrNoRows):
			writeError(w, http.StatusUnauthorized, "not authenticated")
		default:
			log.Printf("update profile: %v", err)
			writeError(w, http.StatusInternalServerError, "could not update profile")
		}
		return
	}

	user, err := db.GetUserByID(r.Context(), a.DB, userID)
	if err != nil {
		log.Printf("get user after update: %v", err)
		writeError(w, http.StatusInternalServerError, "could not load updated profile")
		return
	}
	writeJSON(w, http.StatusOK, userPublic(user))
}

// ---------- POST /api/logout ----------

func (a *Auth) Logout(w http.ResponseWriter, r *http.Request) {
	_ = session.Destroy(r.Context(), a.DB, w, r)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// ---------- helpers ----------

// userPublic returns the user fields safe to expose back to that user
// (it's used by Login / Register / Me / UpdateMe — all self-facing).
// For looking up *other* users we use a slimmer response in users.Get.
func userPublic(u *models.User) map[string]any {
	return map[string]any{
		"id":        u.ID,
		"nickname":  u.Nickname,
		"email":     u.Email,
		"firstName": u.FirstName,
		"lastName":  u.LastName,
		"age":       u.Age,
		"gender":    u.Gender,
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// authedUserID looks up the session cookie and returns the user ID.
// It is the standard guard for handlers that require authentication.
func authedUserID(conn *sql.DB, r *http.Request) (string, error) {
	s, err := session.FromRequest(r.Context(), conn, r)
	if err != nil {
		return "", err
	}
	return s.UserID, nil
}

// requireAuth is the boilerplate-eating wrapper around authedUserID: on
// success it returns (userID, true); on failure it writes a 401 JSON
// response and returns ("", false), so the caller can `if !ok { return }`.
func requireAuth(conn *sql.DB, w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, err := authedUserID(conn, r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "not authenticated")
		return "", false
	}
	return userID, true
}
