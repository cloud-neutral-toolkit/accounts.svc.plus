package auth

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

// OAuthUserProfile represents the unified user profile info from OAuth providers.
type OAuthUserProfile struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// OAuthProvider defines the interface for different OAuth2 providers.
type OAuthProvider interface {
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	FetchProfile(ctx context.Context, token *oauth2.Token) (*OAuthUserProfile, error)
	Name() string
}

type baseProvider struct {
	config *oauth2.Config
	name   string
}

func (p *baseProvider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state)
}

func (p *baseProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.config.Exchange(ctx, code)
}

func (p *baseProvider) Name() string {
	return p.name
}

// GitHubProvider implements GitHub OAuth2.
type GitHubProvider struct {
	baseProvider
}

func NewGitHubProvider(clientID, clientSecret, redirectURL string) *GitHubProvider {
	return &GitHubProvider{
		baseProvider: baseProvider{
			name: "github",
			config: &oauth2.Config{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
				Endpoint:     github.Endpoint,
				Scopes:       []string{"user:email", "read:user"},
			},
		},
	}
}

func (p *GitHubProvider) FetchProfile(ctx context.Context, token *oauth2.Token) (*OAuthUserProfile, error) {
	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
		Login string `json:"login"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	profile := &OAuthUserProfile{
		ID:    fmt.Sprintf("%d", user.ID),
		Email: user.Email,
		Name:  user.Name,
	}
	if profile.Name == "" {
		profile.Name = user.Login
	}

	// GitHub may return empty email if it's private.
	if profile.Email == "" {
		resp, err := client.Get("https://api.github.com/user/emails")
		if err == nil {
			defer resp.Body.Close()
			var emails []struct {
				Email   string `json:"email"`
				Primary bool   `json:"primary"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&emails); err == nil {
				for _, e := range emails {
					if e.Primary {
						profile.Email = e.Email
						break
					}
				}
			}
		}
	}

	return profile, nil
}

// GoogleProvider implements Google OAuth2.
type GoogleProvider struct {
	baseProvider
}

func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{
		baseProvider: baseProvider{
			name: "google",
			config: &oauth2.Config{
				ClientID:     clientID,
				ClientSecret: clientSecret,
				RedirectURL:  redirectURL,
				Endpoint:     google.Endpoint,
				Scopes: []string{
					"https://www.googleapis.com/auth/userinfo.email",
					"https://www.googleapis.com/auth/userinfo.profile",
				},
			},
		},
	}
}

func (p *GoogleProvider) FetchProfile(ctx context.Context, token *oauth2.Token) (*OAuthUserProfile, error) {
	client := p.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var user struct {
		ID    string `json:"id"`
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &OAuthUserProfile{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
	}, nil
}
