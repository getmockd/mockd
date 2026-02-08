package admin

import (
	"encoding/json"
	"net/http"

	"github.com/getmockd/mockd/pkg/store"
)

// getPreferencesStore returns the preferences store to use.
// TODO: Implement admin's own persistent store for preferences.
func (a *API) getPreferencesStore() store.PreferencesStore {
	// TODO: Admin should have its own persistent store for preferences
	// For now, return nil - preferences feature requires persistent storage
	return nil
}

// handleGetPreferences handles GET /preferences.
func (a *API) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	prefsStore := a.getPreferencesStore()
	if prefsStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Preferences management requires persistent storage - coming soon")
		return
	}

	prefs, err := prefsStore.Get(ctx)
	if err != nil {
		a.log.Error("failed to get preferences", "error", err)
		writeError(w, http.StatusInternalServerError, "get_failed", ErrMsgInternalError)
		return
	}

	writeJSON(w, http.StatusOK, prefs)
}

// handleUpdatePreferences handles PUT /preferences.
func (a *API) handleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	prefsStore := a.getPreferencesStore()
	if prefsStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Preferences management requires persistent storage - coming soon")
		return
	}

	var prefs store.Preferences
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", sanitizeJSONError(err, a.log))
		return
	}

	if err := prefsStore.Set(ctx, &prefs); err != nil {
		a.log.Error("failed to update preferences", "error", err)
		writeError(w, http.StatusInternalServerError, "update_failed", ErrMsgInternalError)
		return
	}

	// Return the updated preferences
	writeJSON(w, http.StatusOK, &prefs)
}
