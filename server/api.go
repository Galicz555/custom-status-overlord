package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// ServeHTTP demonstrates a plugin that handles HTTP requests by greeting the world.
// The root URL is currently <siteUrl>/plugins/com.mattermost.plugin-starter-template/api/v1/. Replace com.mattermost.plugin-starter-template with the plugin ID.
const (
	UserPropsKeyCustomStatus = "customStatus"
)

func (p *Plugin) ServeHTTP(c *plugin.Context, w http.ResponseWriter, r *http.Request) {
	router := mux.NewRouter()

	// Middleware to require that the user is logged in
	router.Use(p.MattermostAuthorizationRequired)

	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	apiRouter.HandleFunc("/hello", p.HelloWorld).Methods(http.MethodGet)
	apiRouter.HandleFunc("/custom-status-change", p.CustomStatusChange).Methods(http.MethodPut)

	router.ServeHTTP(w, r)
}

func (p *Plugin) MattermostAuthorizationRequired(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("Mattermost-User-ID")
		if userID == "" {
			http.Error(w, "Not authorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (p *Plugin) HelloWorld(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("Hello, world!")); err != nil {
		p.API.LogError("Failed to write response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (p *Plugin) CustomStatusChange(w http.ResponseWriter, r *http.Request) {
	var cs model.CustomStatus
	if err := json.NewDecoder(r.Body).Decode(&cs); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	userID := r.URL.Query().Get("Mattermost-User-ID")
	if userID == "" {
		http.Error(w, "Not authorized", http.StatusUnauthorized)
		return
	}

	appErr := p.SetCustomStatusWorker(r.Context(), userID, &cs)
	if appErr != nil {
		http.Error(w, appErr.Error(), appErr.StatusCode)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (p *Plugin) SetCustomStatusWorker(ctx context.Context, userID string, cs *model.CustomStatus) *model.AppError {
	if cs == nil || (cs.Emoji == "" && cs.Text == "") {
		return model.NewAppError("SetCustomStatus", "api.custom_status.set_custom_statuses.update.app_error", nil, "", http.StatusBadRequest)
	}

	user, err := p.API.GetUser(userID)
	if err != nil {
		return model.NewAppError("SetCustomStatus", "api.custom_status.set_custom_statuses.user_not_found", nil, "", http.StatusInternalServerError)
	}

	if err := setCustomStatus(user, cs); err != nil {
		p.API.LogError("Failed to set custom status", "userID", userID, "error", err)
		return model.NewAppError("SetCustomStatus", "api.custom_status.set_custom_statuses.set_failed", nil, "", http.StatusInternalServerError)
	}

	_, updateErr := p.API.UpdateUser(user)
	if updateErr != nil {
		p.API.LogError("Failed to update user", "userID", userID, "error", err)
		return model.NewAppError("SetCustomStatus", "api.custom_status.set_custom_statuses.update_failed", nil, "", http.StatusInternalServerError)
	}

	return nil
}

func setCustomStatus(user *model.User, cs *model.CustomStatus) error {
	makeNonNil(user)
	statusJSON, jsonErr := json.Marshal(cs)
	if jsonErr != nil {
		return jsonErr
	}
	user.Props[UserPropsKeyCustomStatus] = string(statusJSON)
	return nil
}

func makeNonNil(user *model.User) {
	if user.Props == nil {
		user.Props = make(map[string]string)
	}

	if user.NotifyProps == nil {
		user.NotifyProps = make(map[string]string)
	}
}
