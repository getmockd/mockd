package admin

import (
	"encoding/json"
	"net/http"

	"github.com/getmockd/mockd/pkg/store"
)

// getPreferencesStore returns the preferences store to use.
// TODO: Implement admin's own persistent store for preferences.
func (a *AdminAPI) getPreferencesStore() store.PreferencesStore {
	// TODO: Admin should have its own persistent store for preferences
	// For now, return nil - preferences feature requires persistent storage
	return nil
}

// handleGetPreferences handles GET /preferences.
func (a *AdminAPI) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	prefsStore := a.getPreferencesStore()
	if prefsStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Preferences management requires persistent storage - coming soon")
		return
	}

	prefs, err := prefsStore.Get(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get_failed", "Failed to get preferences: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, prefs)
}

// handleUpdatePreferences handles PUT /preferences.
func (a *AdminAPI) handleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	prefsStore := a.getPreferencesStore()
	if prefsStore == nil {
		writeError(w, http.StatusNotImplemented, "not_implemented", "Preferences management requires persistent storage - coming soon")
		return
	}

	var prefs store.Preferences
	if err := json.NewDecoder(r.Body).Decode(&prefs); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Failed to parse request body: "+err.Error())
		return
	}

	if err := prefsStore.Set(ctx, &prefs); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", "Failed to update preferences: "+err.Error())
		return
	}

	// Return the updated preferences
	writeJSON(w, http.StatusOK, &prefs)
}
