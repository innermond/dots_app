package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/innermond/dots"
)

func (s *Server) registerAuthRoutes(router *mux.Router) {
	router.HandleFunc("/login", s.handleLogin).Methods("GET")
	router.HandleFunc("/login", s.handleTokening).Methods("POST")
	router.HandleFunc("/logout", s.handleLogout).Methods("GET")
	router.HandleFunc("/oauth/google", s.handleOAuthGoogle).Methods("GET")
	router.HandleFunc("/oauth/google/callback", s.handleOAuthGoogleCallback).Methods("GET")
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	// TODO UI
	ses, err := s.getSession(r)
	if err != nil {
		if err == http.ErrNoCookie {
			s.setSession(w, ses)
		} else {
			Error(w, r, err)
			return
		}
	}

	if !ses.IsZero() {
		http.Redirect(w, r, "/", http.StatusFound)
	} else {
		http.Redirect(w, r, "/oauth/google", http.StatusFound)
	}
}

func (s *Server) handleTokening(w http.ResponseWriter, r *http.Request) {
	cc := dots.TokenCredentials{}
	ok := inputJSON(w, r, &cc, "parse login")
	if !ok {
		return
	}

	str, err := s.TokenService.Create(r.Context(), cc)
	if err != nil {
		Error(w, r, dots.Errorf(dots.EUNAUTHORIZED, "[create token]: %v", err))
		return
	}

	type token struct {
		Access string `json:"token_access"`
	}
	outputJSON(w, r, http.StatusOK, &token{str})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	err := s.setSession(w, Session{})
	if err != nil {
		Error(w, r, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleOAuthGoogle(w http.ResponseWriter, r *http.Request) {
	session, err := s.getSession(r)
	if err != nil {
		Error(w, r, err)
		return
	}

	state := make([]byte, 64)
	_, err = io.ReadFull(rand.Reader, state)
	if err != nil {
		Error(w, r, err)
		return
	}
	session.State = hex.EncodeToString(state)

	err = s.setSession(w, session)
	if err != nil {
		Error(w, r, err)
		return
	}

	authUrl := s.OAuth2Config().AuthCodeURL(session.State)
	http.Redirect(w, r, authUrl, http.StatusFound)
}

func (s *Server) handleOAuthGoogleCallback(w http.ResponseWriter, r *http.Request) {
	session, err := s.getSession(r)
	if err != nil {
		Error(w, r, err)
		return
	}

	code, state := r.FormValue("code"), r.FormValue("state")
	if state != session.State {
		Error(w, r, errors.New("oauth state mismatch"))
	}

	oauth := s.OAuth2Config()
	tok, err := oauth.Exchange(r.Context(), code)
	if err != nil {
		Error(w, r, fmt.Errorf("oauth exchange error: %s", err))
		return
	}

	const oauthGoogleURLAPI = "https://www.googleapis.com/oauth2/v2/userinfo"
	client := oauth.Client(r.Context(), tok)
	resp, err := client.Get(oauthGoogleURLAPI)
	if err != nil {
		Error(w, r, fmt.Errorf("http: response error %s", err))
		return
	}
	cnt, err := io.ReadAll(resp.Body)
	if err != nil {
		Error(w, r, fmt.Errorf("http: cannot read response %s", err))
		return
	}

	type authResponse struct {
		ID            *string `json:"id"`
		Name          *string `json:"name"`
		Login         *string `json:"login"`
		Email         *string `json:"email"`
		VerifiedEmail *bool   `json:"verified_email"`
	}

	var u authResponse
	err = json.Unmarshal(cnt, &u)
	if err != nil {
		Error(w, r, fmt.Errorf("http: cannot decode response %s", err))
		return
	}

	var name string
	if u.Name != nil {
		name = *u.Name
	} else if u.Login != nil {
		name = *u.Login
	}

	var email string
	if u.Email != nil {
		email = *u.Email
	}

	auth := &dots.Auth{
		Source:       dots.AuthSourceGoogle,
		SourceID:     *u.ID,
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		User:         &dots.User{Name: name, Email: email},
	}

	if !tok.Expiry.IsZero() {
		auth.Expiry = &tok.Expiry
	}

	err = s.AuthService.CreateAuth(r.Context(), auth)
	if err != nil {
		Error(w, r, fmt.Errorf("http: cannot create auth: %s", err))
		return
	}

	redirectURL := session.RedirectURL

	session.UserID = auth.UserID
	session.RedirectURL = ""
	session.State = ""
	if err := s.setSession(w, session); err != nil {
		Error(w, r, fmt.Errorf("cannot set session cookie: %s", err))
		return
	}

	if redirectURL == "" {
		redirectURL = "/"
	}
	http.Redirect(w, r, redirectURL, http.StatusFound)

}
