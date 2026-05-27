package handlers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"forum/internal/db"
	"forum/internal/ws"
)

// parseSQLiteTime accepts the half-dozen ways SQLite may emit a datetime
// after losing affinity (e.g. through MAX()): bare "YYYY-MM-DD HH:MM:SS"
// in UTC, or ISO 8601. Returns the first format that succeeds.
func parseSQLiteTime(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unrecognized sqlite time format: " + s)
}

type Users struct {
	DB  *sql.DB
	Hub *ws.Hub
}

// sidebarItem is the JSON shape returned by GET /api/users.
type sidebarItem struct {
	ID            string     `json:"id"`
	Nickname      string     `json:"nickname"`
	Online        bool       `json:"online"`
	HasUnread     bool       `json:"hasUnread"`
	LastMessageAt *time.Time `json:"lastMessageAt"`
}

// List returns all other users for the sidebar, already sorted by:
//   - users with prior messages: by last-message DESC
//   - users with no messages:    alphabetically
//
// Each item carries the current online flag from the WS hub.
func (u *Users) List(w http.ResponseWriter, r *http.Request) {
	selfID, ok := requireAuth(u.DB, w, r)
	if !ok {
		return
	}

	partners, err := db.ListChatPartners(r.Context(), u.DB, selfID)
	if err != nil {
		log.Printf("list chat partners: %v", err)
		writeError(w, http.StatusInternalServerError, "could not load users")
		return
	}

	online := u.Hub.OnlineUsers()
	unread, err := db.ListUnreadPeers(r.Context(), u.DB, selfID)
	if err != nil {
		log.Printf("list unread peers: %v", err)
		unread = map[string]bool{} // sidebar still renders; just no unread dots
	}
	out := make([]sidebarItem, 0, len(partners))
	for _, p := range partners {
		item := sidebarItem{
			ID:        p.ID,
			Nickname:  p.Nickname,
			Online:    online[p.ID],
			HasUnread: unread[p.ID],
		}
		if p.LastMessageAt.Valid {
			if t, err := parseSQLiteTime(p.LastMessageAt.String); err == nil {
				item.LastMessageAt = &t
			}
		}
		out = append(out, item)
	}
	writeJSON(w, http.StatusOK, out)
}

// Get returns the public profile of a single user — what we're comfortable
// showing to any other authenticated user. Email is omitted (private);
// age is omitted to keep the schema-required field out of the public surface.
type userPublicShape struct {
	ID        string    `json:"id"`
	Nickname  string    `json:"nickname"`
	FirstName string    `json:"firstName"`
	LastName  string    `json:"lastName"`
	Gender    string    `json:"gender"`
	CreatedAt time.Time `json:"createdAt"`
	Online    bool      `json:"online"`
}

func (u *Users) Get(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAuth(u.DB, w, r); !ok {
		return
	}
	id := r.PathValue("id")
	user, err := db.GetUserByID(r.Context(), u.DB, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		log.Printf("get user: %v", err)
		writeError(w, http.StatusInternalServerError, "could not load user")
		return
	}
	writeJSON(w, http.StatusOK, userPublicShape{
		ID:        user.ID,
		Nickname:  user.Nickname,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Gender:    user.Gender,
		CreatedAt: user.CreatedAt,
		Online:    u.Hub.OnlineUsers()[user.ID],
	})
}
